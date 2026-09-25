// Package demo встраивает тестовые данные для команды core seed-demo.
package demo

import _ "embed"

//go:embed demo-data.json
var Data []byte
