# Import duplicate detection

The desktop import review checks existing transactions before account/category
mapping is complete. Matches use transaction type, recorded local calendar date,
integer source amount, and exact description. Account, currency, category, tags,
and time of day are deliberately excluded. Description whitespace and case are
significant. Deleted transactions do not match; transfer-in mirror records are
not counted separately.

Duplicate rows appear gray, with a hover tooltip explaining that the transaction
matches an existing entry. They are excluded by default. **Import duplicate
transactions** explicitly overrides this exclusion and restores normal row colors.
Excluded duplicate rows cannot be edited using the pencil or quick dropdowns;
enabling duplicate imports restores those interactions.
The usual validity and row
selection requirements still apply. Matching-field edits trigger a new check;
failed checks block importing and offer Retry.

Matching consumes existing occurrences. If a file has two identical payments and
one exists, one row is marked and the other can be imported. The remaining row
carries its full-file occurrence ordinal when only a subset is submitted. The
server checks current database counts again before saving, so a stale preview
cannot reimport an occurrence saved by an earlier import. API clients that omit
ordinals use the order of their submitted rows. Successful all-duplicate requests
return an imported count of zero.

The server serializes import checks and writes per user within one server process.
Multiple server processes sharing a database are not coordinated by this lock.
Normal transaction creation outside imports also does not acquire this lock.
There is no schema migration or database uniqueness constraint: legitimate
identical payments remain possible through the explicit override.

Content matching cannot distinguish two different real payments with identical
matching fields, and editing an existing description/date/amount can prevent a
later match. Use the override when an excluded row is a separate real payment.

For date-only bank exports, choose the appropriate date format and timezone on
upload. For example, `D10/08/2026` is October 8 with month/day/year, or August 10
with day/month/year. Duplicate detection uses the resulting parsed calendar date.

## Verification

Go regression tests in `pkg/services/import_duplicates_test.go` exercise occurrence
counts, exact fields, timezone boundaries, user/deletion isolation, transfer
mirrors, and pagination. The HTTP smoke test uses a disposable app on port 18086:

```sh
docker run --rm -d --name ezbookkeeping-duplicate-smoke -p 127.0.0.1:18086:8080 ezbookkeeping:duplicate-check
python3 testdata/import_duplicate_smoke.py
docker stop ezbookkeeping-duplicate-smoke
```

The script registers a synthetic user, creates accounts/categories, and verifies
QIF parsing, repeat imports, account-independent matching, explicit override,
occurrence subsets, stale previews, and concurrent imports. Run it only against
this disposable container.

For UI verification, upload the same QIF again and confirm duplicate rows appear
gray before assigning accounts, with an explanatory hover tooltip; select all
and confirm duplicate rows are excluded. Edit a description/amount and check
that the styling refreshes. Enable the override to select duplicate rows.
Check that Import remains disabled during a
check or after a failed check, with Retry available.
