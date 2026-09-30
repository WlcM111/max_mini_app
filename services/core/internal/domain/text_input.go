package domain

import (
	"strings"
	"unicode"
)

// Управляющие символы (категория Unicode Cc) в текстовых полях: NUL не принимает PostgreSQL,
// а одиночный CR ломает построчные форматы вроде .ics (RFC 5545). Такие значения отклоняются
// как VALIDATION_FAILED, а не доходят до хранилища (BUG-003, BUG-004).

// HasControlChars сообщает, есть ли в строке управляющие символы. Для многострочных полей
// (заметки) допустимы перевод строки и табуляция.
func HasControlChars(s string, multiline bool) bool {
	for _, r := range s {
		if !unicode.IsControl(r) {
			continue
		}
		if multiline && (r == '\n' || r == '\t') {
			continue
		}
		return true
	}
	return false
}

// NormalizeMultiline приводит переводы строк к \n: CRLF и одиночный CR из форм и таблиц
// не должны отклоняться как управляющие символы.
func NormalizeMultiline(s string) string {
	return strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(s)
}

// SingleLine заменяет управляющие символы и последовательности пробелов одним пробелом —
// для значений из внешних источников (ответ языковой модели), подставляемых в однострочные поля.
func SingleLine(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}), " ")
}
