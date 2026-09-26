-- credit_ledger_entries.ref_id diperlebar dari CHARACTER(36) ke TEXT:
-- chargeCredit memakai refId komposit (>36 char), mis. "profiler-{campaignId}-{inputHash}"
-- dan "taut:{campaignId}:{mediaId}" → sebelumnya error "value too long for type character(36)".
ALTER TABLE public.credit_ledger_entries ALTER COLUMN ref_id TYPE text;
