package encryption

import (
	"github.com/spf13/viper"
)

// openssl rand -hex 32

const defaultKvEncryptionKey = "e528f7c41f9130f016c68fafdd6bf583ea7f7bb48e4f77ff401afbd81accf8bc"

func getEncryptionKey() string {
	var userProvidedKVEncryptionKey string = viper.GetString("kv.encryptionKey")
	if userProvidedKVEncryptionKey == "" {
		return defaultKvEncryptionKey
	}

	return userProvidedKVEncryptionKey
}
