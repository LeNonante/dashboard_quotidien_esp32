package main

import (
	"log"
	"os"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

// MailCounts est une structure pour stocker le détail et le total
type MailCounts struct {
	Perso int
	Pro   int
	UTT   int
	Total int
}

// getUnreadEmails orchestre la vérification des 3 comptes
func getUnreadEmails() MailCounts {
	counts := MailCounts{}

	// 1. Compte Perso (Gmail)
	counts.Perso = checkSingleAccount(
		"imap.gmail.com:993",
		os.Getenv("EMAIL_USER_PERSO"),
		os.Getenv("EMAIL_PASS_PERSO"),
	)

	// 2. Compte Pro (À adapter si ce n'est pas Gmail, ex: ssl0.ovh.net:993)
	counts.Pro = checkSingleAccount(
		"imap.gmail.com:993",
		os.Getenv("EMAIL_USER_PRO"),
		os.Getenv("EMAIL_PASS_PRO"),
	)

	// 3. Compte Universitaire (À adapter si ce n'est pas Gmail, ex: outlook.office365.com:993)
	counts.UTT = checkSingleAccount(
		"mail.utt.fr:993",
		os.Getenv("EMAIL_USER_UTT"),
		os.Getenv("EMAIL_PASS_UTT"),
	)

	// Calcul du total
	counts.Total = counts.Perso + counts.Pro + counts.UTT

	return counts
}

// checkSingleAccount fait le vrai travail de connexion pour UNE boîte spécifique
func checkSingleAccount(server string, username string, password string) int {
	// Sécurité : on ignore silencieusement si les identifiants manquent dans le .env
	if username == "" || password == "" {
		return 0
	}

	c, err := client.DialTLS(server, nil)
	if err != nil {
		log.Printf("Erreur de connexion IMAP (%s) : %v", username, err)
		return 0
	}
	defer c.Logout()

	if err := c.Login(username, password); err != nil {
		log.Printf("Erreur d'identification IMAP (%s) : %v", username, err)
		return 0
	}

	_, err = c.Select("INBOX", true)
	if err != nil {
		log.Printf("Erreur lors de la sélection de INBOX (%s) : %v", username, err)
		return 0
	}

	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}

	ids, err := c.Search(criteria)
	if err != nil {
		log.Printf("Erreur lors de la recherche des mails (%s) : %v", username, err)
		return 0
	}

	return len(ids)
}
