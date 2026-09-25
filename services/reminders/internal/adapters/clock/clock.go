// Package clock содержит системный источник времени.
package clock

import "time"

// System возвращает текущее время в UTC.
type System struct{}

// Now возвращает текущий момент в UTC.
func (System) Now() time.Time { return time.Now().UTC() }
