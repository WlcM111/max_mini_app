package domain

import "fmt"

// MemberJoinedText — текст сообщения владельцу о новом участнике (spec §15).
func MemberJoinedText(orgName, firstName string, role Role) string {
	return fmt.Sprintf("В организацию «%s» присоединился участник %s с ролью «%s».",
		orgName, firstName, RoleTitle(role))
}

// InviteShareText — текст, который владелец отправляет вместе со ссылкой.
func InviteShareText(orgName string) string {
	return fmt.Sprintf("Приглашаю вести сроки документов «%s» в приложении «Вовремя»", orgName)
}

// MemberJoinedIdempotencyKey — ключ идемпотентности сообщения о новом участнике.
func MemberJoinedIdempotencyKey(invitePublicID string) string { return "mj:" + invitePublicID }

// OpenOrganizationButton — подпись кнопки сообщения о новом участнике.
const OpenOrganizationButton = "Открыть организацию"

// OrganizationPayload формирует payload диплинка организации.
func OrganizationPayload(orgPublicID string) string { return "org_" + orgPublicID }
