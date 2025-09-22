package ledger

// SeedBalance is a test helper that seeds the balance for an account when using the in-memory ledger.
func SeedBalance(l Ledger, code string, amount int64) {
    if mem, ok := l.(*inMemoryLedger); ok {
        mem.mu.Lock()
        defer mem.mu.Unlock()
        mem.balances[code] = amount
    }
}
