package domain

import (
	"html"
	"slices"
	"strings"
	"unicode"

	"github.com/samber/lo"
)

type EmailSettings struct {
	SenderName  string
	SenderEmail string
	FrenchLink  string
	EnglishLink string
}

// RecipientKey identifies a delivery mailbox for reminder throttling only.
// It deliberately leaves contribution matching and member identity unchanged.
func RecipientKey(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func titleCase(s string) string {
	return strings.Join(lo.Map(strings.Fields(s), func(word string, _ int) string {
		runes := []rune(strings.ToLower(word))
		runes[0] = unicode.ToUpper(runes[0])
		return string(runes)
	}), " ")
}

// RenewalEmail renders both alternatives from the same paragraphs.
func RenewalEmail(member Member, settings EmailSettings) Email {
	name := titleCase(member.FirstName)
	french := slices.Contains(member.PreferredLanguages, FrenchLanguage) || member.Country == "France"
	link := settings.EnglishLink
	subject := "Ready for another year with Boavizta? It's time to renew your membership"
	paragraphs := []string{
		"Dear " + name + ",",
		"As your membership with Boavizta comes to an end, we want to say thank you for being with us this past year!",
		"Boavizta exists thanks to the incredible contributions of its members, people like you who help us create and share commons to promote digital practices that respect planetary boundaries. Your involvement really makes a difference.",
		"We're excited about what's coming in 2026 and we will be happy to see you stay involved in our community.",
		"👉 To renew your membership, simply click here: ",
		"Thanks again for being part of Boavizta!",
		"Warm regards,\nBoavizta Team",
	}
	if french {
		link = settings.FrenchLink
		subject = "Prêt pour une nouvelle année avec Boavizta ? Il est temps de renouveler votre adhésion"
		paragraphs = []string{
			"Cher(e) " + name + ",",
			"Alors que votre adhésion à Boavizta touche à sa fin, nous tenons à vous remercier d'avoir été avec nous cette année !",
			"Boavizta existe grâce aux incroyables contributions de ses membres, des personnes comme vous qui nous aident à créer et partager des communs pour promouvoir des pratiques numériques respectueuses des limites planétaires. Votre implication fait vraiment la différence.",
			"Nous sommes enthousiastes à l'idée de ce qui nous attend en 2026 et nous serons heureux de vous voir rester impliqué(e) dans notre communauté.",
			"👉 Pour renouveler votre adhésion, cliquez simplement ici : ",
			"Merci encore de faire partie de Boavizta !",
			"Cordialement,\nL'équipe Boavizta",
		}
	}
	text := slices.Clone(paragraphs)
	text[4] += link
	htmlParagraphs := lo.Map(paragraphs, func(p string, i int) string {
		content := strings.ReplaceAll(html.EscapeString(p), "\n", "<br>")
		if i == 4 {
			content += "<a href=\"" + html.EscapeString(link) + "\">" + html.EscapeString(link) + "</a>"
		}
		return "<p>" + content + "</p>"
	})
	return Email{
		SenderName: settings.SenderName, SenderEmail: settings.SenderEmail,
		ToEmail: member.Email, ToName: name + " " + member.Surname,
		Subject:     subject,
		HtmlContent: "<html><body>" + strings.Join(htmlParagraphs, "") + "</body></html>",
		TextContent: strings.Join(text, "\n\n"),
	}
}
