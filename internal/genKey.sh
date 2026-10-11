#!/bin/bash
randomKey="$(openssl rand -hex 32)"

sed -i "s/\(const key = \)\"[^\"]*\"/\1\"$randomKey\"/" internal/encryption/key.go
