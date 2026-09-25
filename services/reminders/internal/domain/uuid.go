package domain

import "regexp"

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// IsUUID проверяет, что строка — UUID v4 в нижнем регистре (формат openapi.yaml).
func IsUUID(s string) bool { return uuidV4.MatchString(s) }
