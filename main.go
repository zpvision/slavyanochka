package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookie = "slavyanochka_admin"
)

type Event struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Date        string `json:"date"`
	Location    string `json:"location,omitempty"`
	Description string `json:"description"`
}

type GalleryImage struct {
	ID      string `json:"id"`
	File    string `json:"file"`
	Caption string `json:"caption,omitempty"`
	Order   int    `json:"order"`
}

type PastEvent struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Date        string `json:"date"`
	Description string `json:"description"`
	Image       string `json:"image"`
}

type application struct {
	adminUser     string
	adminPassword string
	eventsPath    string
	galleryPath   string
	pastPath      string
	uploadsPath   string
	eventsMu      sync.RWMutex
	galleryMu     sync.RWMutex
	pastMu        sync.RWMutex
	sessionsMu    sync.RWMutex
	sessions      map[string]time.Time
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		log.Fatalf("invalid PORT %q: must be a number between 1 and 65535", port)
	}

	adminUser := os.Getenv("ADMIN_USER")
	if adminUser == "" {
		adminUser = "admin"
	}
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if os.Getenv("APP_ENV") == "production" && adminPassword == "" {
		log.Fatal("ADMIN_PASSWORD must be set when APP_ENV=production")
	}

	app := &application{
		adminUser:     adminUser,
		adminPassword: adminPassword,
		eventsPath:    filepath.Join("data", "events.json"),
		galleryPath:   filepath.Join("data", "gallery.json"),
		pastPath:      filepath.Join("data", "past-events.json"),
		uploadsPath:   filepath.Join("data", "uploads"),
		sessions:      make(map[string]time.Time),
	}
	if err := app.ensureEventsFile(); err != nil {
		log.Fatal(err)
	}
	if err := app.ensureGallery(); err != nil {
		log.Fatal(err)
	}
	if err := app.ensurePastEvents(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/events", app.publicEvents)
	mux.HandleFunc("POST /api/admin/login", app.login)
	mux.HandleFunc("POST /api/admin/logout", app.logout)
	mux.HandleFunc("GET /api/admin/session", app.session)
	mux.HandleFunc("POST /api/admin/events", app.createEvent)
	mux.HandleFunc("PUT /api/admin/events/{id}", app.updateEvent)
	mux.HandleFunc("DELETE /api/admin/events/{id}", app.deleteEvent)
	mux.HandleFunc("GET /api/gallery", app.publicGallery)
	mux.HandleFunc("POST /api/admin/gallery", app.uploadGalleryImage)
	mux.HandleFunc("PUT /api/admin/gallery/order", app.reorderGallery)
	mux.HandleFunc("DELETE /api/admin/gallery/{id}", app.deleteGalleryImage)
	mux.HandleFunc("GET /api/past-events", app.publicPastEvents)
	mux.HandleFunc("POST /api/admin/past-events", app.createPastEvent)
	mux.HandleFunc("PUT /api/admin/past-events/{id}", app.updatePastEvent)
	mux.HandleFunc("DELETE /api/admin/past-events/{id}", app.deletePastEvent)
	mux.HandleFunc("GET /uploads/{file}", app.serveUpload)
	mux.Handle("/", http.FileServer(http.Dir("static")))

	address := ":" + port
	server := &http.Server{Addr: address, Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("Slavyanochka: http://localhost:%s", port)
	log.Printf("Admin: http://localhost:%s/admin/", port)
	log.Fatal(server.ListenAndServe())
}

func (app *application) ensureEventsFile() error {
	if _, err := os.Stat(app.eventsPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	initial := []Event{
		{ID: "summer-festival", Title: "Летний фольклорный фестиваль", Date: "2025-07-26", Location: "Парк Центрального города", Description: "Традиционные песни и танцы"},
		{ID: "harvest-festival", Title: "Праздник урожая", Date: "2025-09-14", Location: "Общественный зал", Description: "Осенний славянский праздник"},
		{ID: "christmas-concert", Title: "Рождественский концерт", Date: "2025-12-20", Location: "Городской культурный центр", Description: "Традиционное рождественское выступление"},
	}
	return app.writeEvents(initial)
}

func (app *application) publicEvents(w http.ResponseWriter, _ *http.Request) {
	events, err := app.readEvents()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить мероприятия")
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (app *application) login(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "Недопустимый источник запроса")
		return
	}
	var credentials struct{ Username, Password string }
	if err := decodeJSON(w, r, &credentials); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	userOK := subtle.ConstantTimeCompare([]byte(credentials.Username), []byte(app.adminUser)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(credentials.Password), []byte(app.adminPassword)) == 1
	if !userOK || !passwordOK {
		writeError(w, http.StatusUnauthorized, "Неверный логин или пароль")
		return
	}
	token, err := randomID(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось создать сессию")
		return
	}
	expires := time.Now().Add(12 * time.Hour)
	app.sessionsMu.Lock()
	app.sessions[token] = expires
	app.sessionsMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: 43200})
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

func (app *application) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		app.sessionsMu.Lock()
		delete(app.sessions, cookie.Value)
		app.sessionsMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (app *application) session(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": app.authenticated(r)})
}

func (app *application) createEvent(w http.ResponseWriter, r *http.Request) {
	if !app.authorizeMutation(w, r) {
		return
	}
	var event Event
	if err := decodeJSON(w, r, &event); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateEvent(&event); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := randomID(8)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось создать мероприятие")
		return
	}
	event.ID = id
	app.eventsMu.Lock()
	defer app.eventsMu.Unlock()
	events, err := app.readEventsUnlocked()
	if err == nil {
		events = append(events, event)
		err = app.writeEventsUnlocked(events)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось сохранить мероприятие")
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (app *application) updateEvent(w http.ResponseWriter, r *http.Request) {
	if !app.authorizeMutation(w, r) {
		return
	}
	var replacement Event
	if err := decodeJSON(w, r, &replacement); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateEvent(&replacement); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	replacement.ID = id
	app.eventsMu.Lock()
	defer app.eventsMu.Unlock()
	events, err := app.readEventsUnlocked()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить мероприятия")
		return
	}
	found := false
	for index := range events {
		if events[index].ID == id {
			events[index] = replacement
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "Мероприятие не найдено")
		return
	}
	if err := app.writeEventsUnlocked(events); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось сохранить изменения")
		return
	}
	writeJSON(w, http.StatusOK, replacement)
}

func (app *application) deleteEvent(w http.ResponseWriter, r *http.Request) {
	if !app.authorizeMutation(w, r) {
		return
	}
	id := r.PathValue("id")
	app.eventsMu.Lock()
	defer app.eventsMu.Unlock()
	events, err := app.readEventsUnlocked()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить мероприятия")
		return
	}
	filtered := events[:0]
	for _, event := range events {
		if event.ID != id {
			filtered = append(filtered, event)
		}
	}
	if len(filtered) == len(events) {
		writeError(w, http.StatusNotFound, "Мероприятие не найдено")
		return
	}
	if err := app.writeEventsUnlocked(filtered); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось удалить мероприятие")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (app *application) ensureGallery() error {
	if err := os.MkdirAll(app.uploadsPath, 0755); err != nil {
		return err
	}
	if _, err := os.Stat(app.galleryPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return app.writeGallery([]GalleryImage{})
}

func (app *application) publicGallery(w http.ResponseWriter, _ *http.Request) {
	images, err := app.readGallery()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить фотографии")
		return
	}
	writeJSON(w, http.StatusOK, images)
}

func (app *application) serveUpload(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("file")
	if filename == "" || filename != filepath.Base(filename) {
		http.NotFound(w, r)
		return
	}
	images, err := app.readGallery()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	allowed := false
	for _, image := range images {
		if image.File == filename {
			allowed = true
			break
		}
	}
	if !allowed {
		pastEvents, pastErr := app.readPastEvents()
		if pastErr == nil {
			for _, event := range pastEvents {
				if event.Image == "/uploads/"+filename {
					allowed = true
					break
				}
			}
		}
	}
	if !allowed {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, filepath.Join(app.uploadsPath, filename))
}

func (app *application) uploadGalleryImage(w http.ResponseWriter, r *http.Request) {
	if !app.authorizeMutation(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "Файл слишком большой. Максимальный размер — 10 МБ")
		return
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Выберите фотографию")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 10<<20+1))
	if err != nil || len(data) == 0 || len(data) > 10<<20 {
		writeError(w, http.StatusBadRequest, "Фотография должна быть не больше 10 МБ")
		return
	}
	extensions := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}
	extension, ok := extensions[http.DetectContentType(data)]
	if !ok {
		writeError(w, http.StatusBadRequest, "Поддерживаются только JPG, PNG и WebP")
		return
	}
	id, err := randomID(12)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось сохранить фотографию")
		return
	}
	filename := id + extension
	if err := os.WriteFile(filepath.Join(app.uploadsPath, filename), data, 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось сохранить фотографию")
		return
	}
	caption := strings.TrimSpace(r.FormValue("caption"))
	if len([]rune(caption)) > 180 {
		_ = os.Remove(filepath.Join(app.uploadsPath, filename))
		writeError(w, http.StatusBadRequest, "Подпись не должна превышать 180 символов")
		return
	}
	app.galleryMu.Lock()
	defer app.galleryMu.Unlock()
	images, err := app.readGalleryUnlocked()
	if err != nil {
		_ = os.Remove(filepath.Join(app.uploadsPath, filename))
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить галерею")
		return
	}
	image := GalleryImage{ID: id, File: filename, Caption: caption, Order: len(images)}
	images = append(images, image)
	if err := app.writeGalleryUnlocked(images); err != nil {
		_ = os.Remove(filepath.Join(app.uploadsPath, filename))
		writeError(w, http.StatusInternalServerError, "Не удалось обновить галерею")
		return
	}
	writeJSON(w, http.StatusCreated, image)
}

func (app *application) reorderGallery(w http.ResponseWriter, r *http.Request) {
	if !app.authorizeMutation(w, r) {
		return
	}
	var payload struct {
		IDs []string `json:"ids"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	app.galleryMu.Lock()
	defer app.galleryMu.Unlock()
	images, err := app.readGalleryUnlocked()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить галерею")
		return
	}
	if len(payload.IDs) != len(images) {
		writeError(w, http.StatusBadRequest, "Некорректный порядок фотографий")
		return
	}
	byID := make(map[string]GalleryImage, len(images))
	for _, image := range images {
		byID[image.ID] = image
	}
	reordered := make([]GalleryImage, 0, len(images))
	for order, id := range payload.IDs {
		image, exists := byID[id]
		if !exists {
			writeError(w, http.StatusBadRequest, "Некорректный порядок фотографий")
			return
		}
		delete(byID, id)
		image.Order = order
		reordered = append(reordered, image)
	}
	if len(byID) != 0 || app.writeGalleryUnlocked(reordered) != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось сохранить порядок")
		return
	}
	writeJSON(w, http.StatusOK, reordered)
}

func (app *application) deleteGalleryImage(w http.ResponseWriter, r *http.Request) {
	if !app.authorizeMutation(w, r) {
		return
	}
	id := r.PathValue("id")
	app.galleryMu.Lock()
	defer app.galleryMu.Unlock()
	images, err := app.readGalleryUnlocked()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить галерею")
		return
	}
	index := -1
	for current := range images {
		if images[current].ID == id {
			index = current
			break
		}
	}
	if index < 0 {
		writeError(w, http.StatusNotFound, "Фотография не найдена")
		return
	}
	removed := images[index]
	images = append(images[:index], images[index+1:]...)
	for order := range images {
		images[order].Order = order
	}
	if err := app.writeGalleryUnlocked(images); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось обновить галерею")
		return
	}
	_ = os.Remove(filepath.Join(app.uploadsPath, removed.File))
	w.WriteHeader(http.StatusNoContent)
}

func (app *application) readGallery() ([]GalleryImage, error) {
	app.galleryMu.RLock()
	defer app.galleryMu.RUnlock()
	return app.readGalleryUnlocked()
}

func (app *application) readGalleryUnlocked() ([]GalleryImage, error) {
	data, err := os.ReadFile(app.galleryPath)
	if err != nil {
		return nil, err
	}
	var images []GalleryImage
	if err := json.Unmarshal(data, &images); err != nil {
		return nil, err
	}
	sort.SliceStable(images, func(i, j int) bool { return images[i].Order < images[j].Order })
	return images, nil
}

func (app *application) writeGallery(images []GalleryImage) error {
	app.galleryMu.Lock()
	defer app.galleryMu.Unlock()
	return app.writeGalleryUnlocked(images)
}

func (app *application) writeGalleryUnlocked(images []GalleryImage) error {
	data, err := json.MarshalIndent(images, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteJSON(app.galleryPath, append(data, '\n'))
}

func (app *application) ensurePastEvents() error {
	if _, err := os.Stat(app.pastPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	initial := []PastEvent{
		{ID: "spring-heritage-2025", Title: "Весенний фестиваль наследия", Date: "2025-04-01", Description: "Концерт традиционной народной музыки Украины, Польши и России.", Image: "/assets/event-1.jpg"},
		{ID: "slavic-culture-2025", Title: "Вечер славянской культуры", Date: "2025-02-01", Description: "Выступление для местной славянской общины с хоровой и танцевальной программой.", Image: "/assets/event-2.jpg"},
		{ID: "christmas-2024", Title: "Рождественский народный концерт", Date: "2024-12-01", Description: "Праздничный концерт с традиционными рождественскими песнями и семейным торжеством.", Image: "/assets/event-3.jpg"},
	}
	return app.writePastEvents(initial)
}

func (app *application) publicPastEvents(w http.ResponseWriter, _ *http.Request) {
	events, err := app.readPastEvents()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить прошедшие мероприятия")
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (app *application) createPastEvent(w http.ResponseWriter, r *http.Request) {
	if !app.authorizeMutation(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "Файл слишком большой. Максимальный размер — 10 МБ")
		return
	}
	image, err := app.saveFormImage(r, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := randomID(12)
	if err != nil {
		_ = app.removeUploadedImage(image)
		writeError(w, http.StatusInternalServerError, "Не удалось создать мероприятие")
		return
	}
	event := PastEvent{ID: id, Title: strings.TrimSpace(r.FormValue("title")), Date: strings.TrimSpace(r.FormValue("date")), Description: strings.TrimSpace(r.FormValue("description")), Image: image}
	if err := validatePastEvent(&event); err != nil {
		_ = app.removeUploadedImage(image)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	app.pastMu.Lock()
	defer app.pastMu.Unlock()
	events, err := app.readPastEventsUnlocked()
	if err == nil {
		events = append(events, event)
		err = app.writePastEventsUnlocked(events)
	}
	if err != nil {
		_ = app.removeUploadedImage(image)
		writeError(w, http.StatusInternalServerError, "Не удалось сохранить мероприятие")
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (app *application) updatePastEvent(w http.ResponseWriter, r *http.Request) {
	if !app.authorizeMutation(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "Файл слишком большой. Максимальный размер — 10 МБ")
		return
	}
	newImage, err := app.saveFormImage(r, false)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	app.pastMu.Lock()
	defer app.pastMu.Unlock()
	events, err := app.readPastEventsUnlocked()
	if err != nil {
		_ = app.removeUploadedImage(newImage)
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить мероприятия")
		return
	}
	index := -1
	for current := range events {
		if events[current].ID == r.PathValue("id") {
			index = current
			break
		}
	}
	if index < 0 {
		_ = app.removeUploadedImage(newImage)
		writeError(w, http.StatusNotFound, "Мероприятие не найдено")
		return
	}
	oldImage := events[index].Image
	replacement := PastEvent{ID: events[index].ID, Title: strings.TrimSpace(r.FormValue("title")), Date: strings.TrimSpace(r.FormValue("date")), Description: strings.TrimSpace(r.FormValue("description")), Image: oldImage}
	if newImage != "" {
		replacement.Image = newImage
	}
	if err := validatePastEvent(&replacement); err != nil {
		_ = app.removeUploadedImage(newImage)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	events[index] = replacement
	if err := app.writePastEventsUnlocked(events); err != nil {
		_ = app.removeUploadedImage(newImage)
		writeError(w, http.StatusInternalServerError, "Не удалось сохранить изменения")
		return
	}
	if newImage != "" {
		_ = app.removeUploadedImage(oldImage)
	}
	writeJSON(w, http.StatusOK, replacement)
}

func (app *application) deletePastEvent(w http.ResponseWriter, r *http.Request) {
	if !app.authorizeMutation(w, r) {
		return
	}
	app.pastMu.Lock()
	defer app.pastMu.Unlock()
	events, err := app.readPastEventsUnlocked()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить мероприятия")
		return
	}
	index := -1
	for current := range events {
		if events[current].ID == r.PathValue("id") {
			index = current
			break
		}
	}
	if index < 0 {
		writeError(w, http.StatusNotFound, "Мероприятие не найдено")
		return
	}
	removed := events[index]
	events = append(events[:index], events[index+1:]...)
	if err := app.writePastEventsUnlocked(events); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось удалить мероприятие")
		return
	}
	_ = app.removeUploadedImage(removed.Image)
	w.WriteHeader(http.StatusNoContent)
}

func (app *application) saveFormImage(r *http.Request, required bool) (string, error) {
	file, _, err := r.FormFile("image")
	if errors.Is(err, http.ErrMissingFile) && !required {
		return "", nil
	}
	if err != nil {
		return "", errors.New("выберите фотографию")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (10<<20)+1))
	if err != nil || len(data) == 0 || len(data) > 10<<20 {
		return "", errors.New("фотография должна быть не больше 10 МБ")
	}
	extensions := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}
	extension, ok := extensions[http.DetectContentType(data)]
	if !ok {
		return "", errors.New("поддерживаются только JPG, PNG и WebP")
	}
	id, err := randomID(12)
	if err != nil {
		return "", errors.New("не удалось сохранить фотографию")
	}
	filename := id + extension
	if err := os.WriteFile(filepath.Join(app.uploadsPath, filename), data, 0644); err != nil {
		return "", errors.New("не удалось сохранить фотографию")
	}
	return "/uploads/" + filename, nil
}

func (app *application) removeUploadedImage(image string) error {
	const prefix = "/uploads/"
	if !strings.HasPrefix(image, prefix) {
		return nil
	}
	filename := strings.TrimPrefix(image, prefix)
	if filename == "" || filename != filepath.Base(filename) {
		return nil
	}
	return os.Remove(filepath.Join(app.uploadsPath, filename))
}

func validatePastEvent(event *PastEvent) error {
	if event.Title == "" || event.Date == "" || event.Description == "" || event.Image == "" {
		return errors.New("заполните фотографию, название, дату и описание")
	}
	if len([]rune(event.Title)) > 120 || len([]rune(event.Description)) > 600 {
		return errors.New("одно из полей превышает допустимую длину")
	}
	if _, err := time.Parse("2006-01-02", event.Date); err != nil {
		return errors.New("укажите корректную дату")
	}
	return nil
}

func (app *application) readPastEvents() ([]PastEvent, error) {
	app.pastMu.RLock()
	defer app.pastMu.RUnlock()
	return app.readPastEventsUnlocked()
}

func (app *application) readPastEventsUnlocked() ([]PastEvent, error) {
	data, err := os.ReadFile(app.pastPath)
	if err != nil {
		return nil, err
	}
	var events []PastEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Date > events[j].Date })
	return events, nil
}

func (app *application) writePastEvents(events []PastEvent) error {
	app.pastMu.Lock()
	defer app.pastMu.Unlock()
	return app.writePastEventsUnlocked(events)
}

func (app *application) writePastEventsUnlocked(events []PastEvent) error {
	sort.SliceStable(events, func(i, j int) bool { return events[i].Date > events[j].Date })
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteJSON(app.pastPath, append(data, '\n'))
}

func (app *application) authorizeMutation(w http.ResponseWriter, r *http.Request) bool {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "Недопустимый источник запроса")
		return false
	}
	if !app.authenticated(r) {
		writeError(w, http.StatusUnauthorized, "Требуется вход в админку")
		return false
	}
	return true
}

func (app *application) authenticated(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	app.sessionsMu.RLock()
	expires, ok := app.sessions[cookie.Value]
	app.sessionsMu.RUnlock()
	if !ok || time.Now().After(expires) {
		if ok {
			app.sessionsMu.Lock()
			delete(app.sessions, cookie.Value)
			app.sessionsMu.Unlock()
		}
		return false
	}
	return true
}

func (app *application) readEvents() ([]Event, error) {
	app.eventsMu.RLock()
	defer app.eventsMu.RUnlock()
	return app.readEventsUnlocked()
}

func (app *application) readEventsUnlocked() ([]Event, error) {
	data, err := os.ReadFile(app.eventsPath)
	if err != nil {
		return nil, err
	}
	var events []Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Date < events[j].Date })
	return events, nil
}

func (app *application) writeEvents(events []Event) error {
	app.eventsMu.Lock()
	defer app.eventsMu.Unlock()
	return app.writeEventsUnlocked(events)
}

func (app *application) writeEventsUnlocked(events []Event) error {
	sort.SliceStable(events, func(i, j int) bool { return events[i].Date < events[j].Date })
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(app.eventsPath), 0755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(app.eventsPath), "events-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	backup := app.eventsPath + ".bak"
	_ = os.Remove(backup)
	if _, statErr := os.Stat(app.eventsPath); statErr == nil {
		if err := os.Rename(app.eventsPath, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(temporaryPath, app.eventsPath); err != nil {
		_ = os.Rename(backup, app.eventsPath)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func validateEvent(event *Event) error {
	event.Title = strings.TrimSpace(event.Title)
	event.Date = strings.TrimSpace(event.Date)
	event.Location = strings.TrimSpace(event.Location)
	event.Description = strings.TrimSpace(event.Description)
	if event.Title == "" || event.Date == "" || event.Description == "" {
		return errors.New("Заполните название, дату и описание")
	}
	if len([]rune(event.Title)) > 120 || len([]rune(event.Location)) > 160 || len([]rune(event.Description)) > 600 {
		return errors.New("Одно из полей превышает допустимую длину")
	}
	if _, err := time.Parse("2006-01-02", event.Date); err != nil {
		return errors.New("Укажите корректную дату")
	}
	return nil
}

func atomicWriteJSON(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	backup := path + ".bak"
	_ = os.Remove(backup)
	if _, statErr := os.Stat(path); statErr == nil {
		if err := os.Rename(path, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Rename(backup, path)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func randomID(bytesCount int) (string, error) {
	value := make([]byte, bytesCount)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == "" || origin == "http://"+r.Host || origin == "https://"+r.Host
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("Некорректные данные: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
