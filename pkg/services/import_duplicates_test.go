package services

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/stretchr/testify/require"
)

func importCandidate(comment string) *models.ImportDuplicateCandidate {
	return &models.ImportDuplicateCandidate{Type: models.TRANSACTION_TYPE_EXPENSE, Time: time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC).Unix(), UtcOffset: 120, Amount: 1300, Comment: comment}
}

func TestImportDuplicateOccurrenceMatching(t *testing.T) {
	row := importCandidate("PURCH Interpark Cavendish")
	counts := map[importDuplicateKey]int{duplicateKey(row): 1}
	matches, err := matchImportDuplicates([]*models.ImportDuplicateCandidate{row, row}, counts, nil)
	require.NoError(t, err)
	require.True(t, matches[0].Duplicate)
	require.False(t, matches[1].Duplicate)
	// Submitting just the second row retains the preview's occurrence context.
	matches, err = matchImportDuplicates([]*models.ImportDuplicateCandidate{row}, counts, []int{2})
	require.NoError(t, err)
	require.False(t, matches[0].Duplicate)
	// A stale preview cannot reimport it after another request saves occurrence 2.
	counts[duplicateKey(row)] = 2
	matches, err = matchImportDuplicates([]*models.ImportDuplicateCandidate{row}, counts, []int{2})
	require.NoError(t, err)
	require.True(t, matches[0].Duplicate)
	for _, ordinals := range [][]int{{0}, {65536}, {1, 1}} {
		_, err = matchImportDuplicates([]*models.ImportDuplicateCandidate{row, row}, counts, ordinals)
		require.Error(t, err)
	}
}

func TestImportDuplicateExactFieldsAndLocalDate(t *testing.T) {
	row := importCandidate("A|B  merchant")
	existing := map[importDuplicateKey]int{duplicateKey(row): 1}
	sameDate := *row
	sameDate.Time += 10 * 60 * 60
	differentDate := *row
	differentDate.Time += 24 * 60 * 60
	differentDescription := *row
	differentDescription.Comment = "a|b  merchant"
	differentAmount := *row
	differentAmount.Amount++
	differentType := *row
	differentType.Type = models.TRANSACTION_TYPE_INCOME
	matches, err := matchImportDuplicates([]*models.ImportDuplicateCandidate{&sameDate, &differentDate, &differentDescription, &differentAmount, &differentType}, existing, nil)
	require.NoError(t, err)
	require.True(t, matches[0].Duplicate)
	for _, match := range matches[1:] {
		require.False(t, match.Duplicate)
	}
	// The UTC day differs, but the recorded local date still matches.
	differentOffset := *row
	differentOffset.Time -= 3 * 60 * 60
	differentOffset.UtcOffset = 300
	require.Equal(t, duplicateKey(row), duplicateKey(&differentOffset))
}

func TestImportDuplicateDatabaseScopeAndPagination(t *testing.T) {
	previous := *datastore.Container
	defer func() { *datastore.Container = previous }()
	require.NoError(t, datastore.InitializeDataStore(&settings.Config{DatabaseConfig: &settings.DatabaseConfig{
		DatabaseType: settings.Sqlite3DbType, DatabasePath: filepath.Join(t.TempDir(), "duplicates.db"), MaxIdleConnection: 1, MaxOpenConnection: 1,
	}}))
	require.NoError(t, datastore.Container.UserDataStore.SyncStructs(new(models.Transaction)))
	service := &TransactionService{ServiceUsingDB: ServiceUsingDB{container: datastore.Container}}
	row := importCandidate("bank memo")
	for i := 0; i < 1005; i++ {
		// More than one query page; unrelated descriptions must not consume matches.
		record := &models.Transaction{TransactionId: int64(i + 1), Uid: 1, Type: models.TRANSACTION_DB_TYPE_EXPENSE,
			TransactionTime: utils.GetMinTransactionTimeFromUnixTime(row.Time) + int64(i), TimezoneUtcOffset: row.UtcOffset, Amount: row.Amount, Comment: "other", AccountId: 999}
		if i == 0 {
			record.Comment = row.Comment
		}
		if i == 1 {
			record.Comment = row.Comment
			record.Deleted = true
		}
		if i == 2 {
			record.Comment = row.Comment
			record.Uid = 2
		}
		if i == 3 {
			record.Comment = row.Comment
			record.Type = models.TRANSACTION_DB_TYPE_TRANSFER_IN
		}
		sess := service.UserDataDB(1).NewSession(core.NewNullContext())
		_, err := sess.Insert(record)
		sess.Close()
		require.NoError(t, err)
	}
	matches, err := service.CheckImportDuplicates(core.NewNullContext(), 1, []*models.ImportDuplicateCandidate{row, row}, nil)
	require.NoError(t, err)
	require.True(t, matches[0].Duplicate)
	require.False(t, matches[1].Duplicate)
	matches, err = service.CheckImportDuplicates(core.NewNullContext(), 2, []*models.ImportDuplicateCandidate{row}, nil)
	require.NoError(t, err)
	require.True(t, matches[0].Duplicate)
	// Transfer-in mirrors are never counted, even for transfer candidates.
	transfer := *row
	transfer.Type = models.TRANSACTION_TYPE_TRANSFER
	matches, err = service.CheckImportDuplicates(core.NewNullContext(), 1, []*models.ImportDuplicateCandidate{&transfer}, nil)
	require.NoError(t, err)
	require.False(t, matches[0].Duplicate)
	// Same local date at opposite timezone extremes can be almost 50 UTC hours apart.
	extreme := importCandidate("timezone boundary")
	extreme.Time = time.Date(2026, 10, 6, 0, 0, 0, 0, time.FixedZone("east", 14*3600)).Unix()
	extreme.UtcOffset = 840
	record := &models.Transaction{TransactionId: 2000, Uid: 1, Type: models.TRANSACTION_DB_TYPE_EXPENSE,
		TransactionTime:   utils.GetMinTransactionTimeFromUnixTime(time.Date(2026, 10, 6, 23, 59, 0, 0, time.FixedZone("west", -12*3600)).Unix()),
		TimezoneUtcOffset: -720, Amount: extreme.Amount, Comment: extreme.Comment}
	sess := service.UserDataDB(1).NewSession(core.NewNullContext())
	_, err = sess.Insert(record)
	sess.Close()
	require.NoError(t, err)
	matches, err = service.CheckImportDuplicates(core.NewNullContext(), 1, []*models.ImportDuplicateCandidate{extreme}, nil)
	require.NoError(t, err)
	require.True(t, matches[0].Duplicate)

}
