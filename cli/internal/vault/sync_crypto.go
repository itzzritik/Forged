package vault

// Sync blobs use the Symmetric Key directly via EncryptCombined/DecryptCombined (defined in crypto.go).
// The Protected Symmetric Key pattern replaces the old HKDF sync key derivation.
// ExportForSync and ImportFromSync in vault.go call EncryptCombined/DecryptCombined directly.
