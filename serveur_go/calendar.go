package main

import (
	"context"
	"log"
	"sort"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// EventInfo est une structure sur-mesure pour stocker les infos essentielles
// de n'importe quel agenda avant de les trier.
type EventInfo struct {
	StartTime time.Time
	TimeStr   string
	Summary   string
	Agenda    string // Pratique pour afficher un petit tag "Pro" ou "Perso"
}

func getGoogleCalendarEvents() []EventInfo {
	ctx := context.Background()

	// 1. Authentification
	calendarService, err := calendar.NewService(ctx, option.WithCredentialsFile("credentials.json"))
	if err != nil {
		log.Fatalf("Impossible de créer le client Google Calendar: %v", err)
	}

	// 2. Plage horaire (de minuit à 23h59 aujourd'hui)
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Format(time.RFC3339)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Format(time.RFC3339)

	// 3. Déclaration de tes 3 agendas
	// On utilise une map : la clé est l'ID, la valeur est le nom que tu veux lui donner
	agendas := map[string]string{
		"b85ef55c7d755fa46720fe09471c4b0e0d836e647666317379e6185b42eedfbf@group.calendar.google.com": "Perso",
		"658253c94af54f208b10aca612100ff3d8d549ee573a30b893870c29777cca6c@group.calendar.google.com": "Emploie du temps UTT",
		"8879e3e8c1759f792ff709ea0c1c3d4bce707942a5224db1e908a731d99c18b7@group.calendar.google.com": "UTT - Pro",
	}

	var allEvents []EventInfo

	// 4. Boucle de récupération
	for calendarId, agendaName := range agendas {
		events, err := calendarService.Events.List(calendarId).
			ShowDeleted(false).
			SingleEvents(true).
			TimeMin(startOfDay).
			TimeMax(endOfDay).
			OrderBy("startTime").
			Do()

		if err != nil {
			log.Printf("Erreur lors de la lecture de l'agenda %s: %v", agendaName, err)
			continue // S'il y a un souci avec un agenda, on ne fait pas planter les autres
		}

		for _, item := range events.Items {
			var startTime time.Time
			var timeStr string

			if item.Start.DateTime == "" {
				// C'est un événement sur la journée entière (pas d'heure précise)
				startTime, _ = time.Parse("2006-01-02", item.Start.Date)
				timeStr = "Journée"
			} else {
				// C'est un événement classique avec une heure
				startTime, _ = time.Parse(time.RFC3339, item.Start.DateTime)
				timeStr = startTime.Format("15:04")
			}

			// On ajoute l'événement à notre liste globale
			allEvents = append(allEvents, EventInfo{
				StartTime: startTime,
				TimeStr:   timeStr,
				Summary:   item.Summary,
				Agenda:    agendaName,
			})
		}
	}

	// 5. Tri chronologique de la liste globale
	// On utilise le package "sort" natif de Go pour comparer les heures de début
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].StartTime.Before(allEvents[j].StartTime)
	})

	// 6. Renvoie des events triés pour l'affichage dans le template HTML
	return allEvents
}
