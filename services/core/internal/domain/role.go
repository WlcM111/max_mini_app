package domain

// Role — роль участника организации.
type Role string

// Роли по возрастанию прав.
const (
	RoleViewer Role = "viewer"
	RoleEditor Role = "editor"
	RoleOwner  Role = "owner"
)

// rank возвращает порядок роли: viewer < editor < owner.
func (r Role) rank() int {
	switch r {
	case RoleViewer:
		return 1
	case RoleEditor:
		return 2
	case RoleOwner:
		return 3
	default:
		return 0
	}
}

// Allows сообщает, достаточно ли роли для действия, требующего min.
func (r Role) Allows(min Role) bool { return r.rank() >= min.rank() && r.rank() > 0 }

// Valid сообщает, входит ли значение в словарь ролей.
func (r Role) Valid() bool { return r.rank() > 0 }

// Invitable сообщает, можно ли выдать роль приглашением или сменой роли участника.
func (r Role) Invitable() bool { return r == RoleEditor || r == RoleViewer }

// ParseRole разбирает роль из внешнего представления.
func ParseRole(s string) (Role, bool) {
	r := Role(s)
	return r, r.Valid()
}

// RoleTitle — родительный падеж названия роли для текста сообщения.
func RoleTitle(r Role) string {
	switch r {
	case RoleOwner:
		return "владелец"
	case RoleEditor:
		return "редактор"
	default:
		return "наблюдатель"
	}
}
