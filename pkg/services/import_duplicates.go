package services

import (
	"sync"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// Serialize imports per user in this server process, including the final check.
var importLocks [64]sync.Mutex

func LockTransactionImport(uid int64) func() {
	lock := &importLocks[uint64(uid)%uint64(len(importLocks))]
	lock.Lock()
	return lock.Unlock
}

type importDuplicateKey struct {
	Type    models.TransactionType
	Date    string
	Amount  int64
	Comment string
}

func duplicateKey(candidate *models.ImportDuplicateCandidate) importDuplicateKey {
	local := time.Unix(candidate.Time, 0).In(time.FixedZone("transaction", int(candidate.UtcOffset)*60))
	return importDuplicateKey{candidate.Type, local.Format("2006-01-02"), candidate.Amount, candidate.Comment}
}

// matchImportDuplicates consumes existing occurrences rather than discarding all
// identical rows. Explicit ordinals preserve the preview's full-file context when
// only some rows are selected for submission.
func matchImportDuplicates(candidates []*models.ImportDuplicateCandidate, existing map[importDuplicateKey]int, occurrences []int) ([]*models.ImportDuplicateMatch, error) {
	if len(occurrences) != 0 && len(occurrences) != len(candidates) {
		return nil, errs.ErrParameterInvalid
	}
	counts := make(map[importDuplicateKey]int)
	used := make(map[importDuplicateKey]map[int]bool)
	matches := make([]*models.ImportDuplicateMatch, len(candidates))
	for i, candidate := range candidates {
		if candidate == nil {
			return nil, errs.ErrParameterInvalid
		}
		key := duplicateKey(candidate)
		counts[key]++
		ordinal := counts[key]
		if len(occurrences) > 0 {
			ordinal = occurrences[i]
		}
		if ordinal < 1 || ordinal > 65535 {
			return nil, errs.ErrParameterInvalid
		}
		if used[key] == nil {
			used[key] = make(map[int]bool)
		}
		if used[key][ordinal] {
			return nil, errs.ErrParameterInvalid
		}
		used[key][ordinal] = true
		matches[i] = &models.ImportDuplicateMatch{Duplicate: ordinal <= existing[key], Occurrence: ordinal}
	}
	return matches, nil
}

// CheckImportDuplicates scans only the incoming date range and excludes deleted
// records and transfer-in mirror rows. Exact comparison happens in Go, avoiding
// database collation differences for descriptions.
func (s *TransactionService) CheckImportDuplicates(c core.Context, uid int64, candidates []*models.ImportDuplicateCandidate, occurrences []int) ([]*models.ImportDuplicateMatch, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	if len(candidates) > 65535 {
		return nil, errs.ErrImportTooManyTransaction
	}
	if len(candidates) == 0 {
		return []*models.ImportDuplicateMatch{}, nil
	}
	if candidates[0] == nil {
		return nil, errs.ErrParameterInvalid
	}
	minTime, maxTime := candidates[0].Time, candidates[0].Time
	wanted := make(map[importDuplicateKey]bool)
	for _, candidate := range candidates {
		if candidate == nil {
			return nil, errs.ErrParameterInvalid
		}
		if candidate.Time < minTime {
			minTime = candidate.Time
		}
		if candidate.Time > maxTime {
			maxTime = candidate.Time
		}
		wanted[duplicateKey(candidate)] = true
	}
	// Same-date records can differ by almost 50 hours in UTC (+14 versus
	// -12 offsets, plus different times of day). Pad by three days.
	lower := utils.GetMinTransactionTimeFromUnixTime(minTime - 72*60*60)
	upper := utils.GetMaxTransactionTimeFromUnixTime(maxTime + 72*60*60)
	existing := make(map[importDuplicateKey]int)
	for {
		var records []*models.Transaction
		sess := s.UserDataDB(uid).NewSession(c)
		err := sess.Where("uid=? AND deleted=? AND type<>? AND transaction_time>=? AND transaction_time<=?", uid, false, models.TRANSACTION_DB_TYPE_TRANSFER_IN, lower, upper).
			Desc("transaction_time").Limit(1000).Find(&records)
		sess.Close()
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			typ, err := record.Type.ToTransactionType()
			if err != nil {
				continue
			}
			key := duplicateKey(&models.ImportDuplicateCandidate{Type: typ, Time: utils.GetUnixTimeFromTransactionTime(record.TransactionTime), UtcOffset: record.TimezoneUtcOffset, Amount: record.Amount, Comment: record.Comment})
			if wanted[key] {
				existing[key]++
			}
		}
		if len(records) < 1000 {
			break
		}
		upper = records[len(records)-1].TransactionTime - 1
	}
	return matchImportDuplicates(candidates, existing, occurrences)
}
