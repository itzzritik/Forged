package vault

// Sync blobs use the Symmetric Key directly via EncryptCombined/DecryptCombined (defined in crypto.go).
// The Protected Symmetric Key pattern replaces the old HKDF sync key derivation.
// ExportForSync in vault.go calls EncryptCombined directly.
