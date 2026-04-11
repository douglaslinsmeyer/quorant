-- Prevent DELETE on ledger_entries (corrections via adjustment entries only).
CREATE OR REPLACE FUNCTION prevent_ledger_delete() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'ledger_entries are append-only; use adjustment entries for corrections';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ledger_entries_no_delete
    BEFORE DELETE ON ledger_entries
    FOR EACH ROW EXECUTE FUNCTION prevent_ledger_delete();

-- Prevent DELETE on fund_transactions (corrections via reversal transactions only).
CREATE OR REPLACE FUNCTION prevent_fund_tx_delete() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'fund_transactions are append-only; use reversal transactions for corrections';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_fund_transactions_no_delete
    BEFORE DELETE ON fund_transactions
    FOR EACH ROW EXECUTE FUNCTION prevent_fund_tx_delete();
