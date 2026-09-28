package domain

// RecipientsFor возвращает получателей напоминаний документа: только ответственного,
// если он назначен и получает напоминания; иначе всех участников, как раньше.
func RecipientsFor(doc Document, members []Member) []Member {
	if doc.ResponsibleAccountID == "" {
		return members
	}
	for _, m := range members {
		if m.AccountID == doc.ResponsibleAccountID && m.Receives() {
			return []Member{m}
		}
	}
	return members
}
