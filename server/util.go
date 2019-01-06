package server

import (
	json "encoding/json"
	errors "errors"
	impress "github.com/DanInci/raspberry-projector/impress"
	betterguid "github.com/kjk/betterguid"
	http "net/http"
	strconv "strconv"
)

func isSlideShowOwnerUUID(uuid string) bool {
	if isSlideShowRunning() {
		return getImpressClient().GetPresentationUUID() == uuid
	}
	return false
}

func isSlideShowRunning() bool {
	client := getImpressClient()
	return client != nil && !client.IsTerminated()
}

func generateUUID() string {
	return betterguid.New()
}

func encodeImpressStats(impressStats *impress.ImpressStats) ([]byte, error) {
	statusEncoding := make(map[string]interface{})

	if len(impressStats.Status) > 0 {
		statusEncoding["command"] = impressStats.Status[0]
		switch impressStats.Status[0] {
		case impress.SLIDE_SHOW_FINISHED:
		case impress.SLIDE_SHOW_STARTED:
			totalSlides, _ := strconv.Atoi(impressStats.Status[1])
			currentSlide, _ := strconv.Atoi(impressStats.Status[2])
			statusEncoding["totalSlides"] = totalSlides
			statusEncoding["currentSlide"] = currentSlide
		case impress.SLIDE_UPDATED:
			currentSlide, _ := strconv.Atoi(impressStats.Status[1])
			statusEncoding["currentSlide"] = currentSlide
		default:
			return nil, errors.New("Failed to encode command")
		}
	}

	response := map[string]interface{}{
		"name":           impressStats.Name,
		"status":         statusEncoding,
		"controllers":    impressStats.Controllers,
		"maxControllers": impressStats.MaxControllers,
		"isOwnerPresent": impressStats.IsOwnerPresent,
		"ownerTimeout":   impressStats.OwnerTimeout,
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func writeError(w http.ResponseWriter, message string, status int) {
	toEncode := map[string]string{
		"error": message,
	}
	encoded, _ := json.Marshal(toEncode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(encoded)
}
