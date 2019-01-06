package impress

import (
	json "encoding/json"
	errors "errors"
	websocket "github.com/gorilla/websocket"
	strconv "strconv"
	strings "strings"
	time "time"
)

const (
	PAIR_WITH_SERVER = "LO_SERVER_CLIENT_PAIR"

	TRANSITION_NEXT           = "transition_next"
	TRANSITION_PREVIOUS       = "transition_previous"
	GO_TO_SLIDE               = "goto_slide"
	PRESENTATION_BLANK_SCREEN = "presentation_blank_screen"
	PRESENTATION_RESUME       = "presentation_resume"
	PRESENTATION_START        = "presentation_start"
	PRESENTATION_STOP         = "presentation_stop"

	POINTER_STARTED      = "pointer_started"
	POINTER_COORDINATION = "pointer_coordination"
	POINTER_DISMISSED    = "pointer_dismissed"
)

type ImpressController struct {
	conn    *websocket.Conn
	isOwner bool
	send    chan []string
}

const (
	writeWait       = 10 * time.Second
	pongWait        = 60 * time.Second
	pingPeriod      = (pongWait * 9) / 10
	readBufferSize  = 1024
	writeBufferSize = 1024
)

func NewController(socket *websocket.Conn, isOwner bool) *ImpressController {
	controller := &ImpressController{conn: socket, isOwner: isOwner, send: make(chan []string)}
	return controller
}

func (c *ImpressController) IsOwner() bool {
	return c.isOwner
}

func (c *ImpressController) StartPumping(client *ImpressClient) {
	go c.readPump(client)
	go c.writePump()
}

func (controller *ImpressController) readPump(client *ImpressClient) {
	defer func() {
		client.unregister <- controller
		controller.conn.Close()
	}()
	client.register <- controller
	controller.conn.SetReadLimit(readBufferSize)
	controller.conn.SetReadDeadline(time.Now().Add(pongWait))
	controller.conn.SetPongHandler(func(string) error { controller.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := controller.conn.ReadMessage()
		if err != nil {
			break
		}
		request, err2 := decodeRequest(message)
		if err2 != nil {
			controller.writeError(err2.Error())
			continue
		}

		if request[0] == PRESENTATION_STOP && !controller.IsOwner() {
			controller.writeError("Only the owner can terminate the session")
			continue
		}

		client.requests <- request
	}
}

func decodeRequest(body []byte) ([]string, error) {
	var decoded map[string]string
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, errors.New("Malformed JSON syntax")
	}
	value, ok := decoded["command"]
	if !ok {
		return nil, errors.New("command key not found")
	}
	switch value {
	case TRANSITION_NEXT, TRANSITION_PREVIOUS, PRESENTATION_BLANK_SCREEN, PRESENTATION_RESUME, PRESENTATION_START, PRESENTATION_STOP:
	case GO_TO_SLIDE:
		value, ok := decoded["index"]
		if !ok {
			return nil, errors.New("index key required")
		}
		conv, err := strconv.Atoi(value)
		if err != nil || conv < 0 {
			return nil, errors.New("index value not a number or less than 0")
		}
	default:
		return nil, errors.New("command not recognized")
	}

	request := make([]string, 0, len(decoded))
	for _, value := range decoded {
		request = append(request, value)
	}
	return request, nil
}

func (controller *ImpressController) writeError(message string) {
	w, err := controller.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return
	}

	toEncode := make(map[string]interface{})
	toEncode["error"] = message
	encoded, _ := json.Marshal(toEncode)
	w.Write(encoded)

	if err := w.Close(); err != nil {
		return
	}
}

func (controller *ImpressController) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		controller.conn.Close()
	}()
	for {
		select {
		case message, ok := <-controller.send:
			controller.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				controller.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err1 := controller.conn.NextWriter(websocket.TextMessage)
			if err1 != nil {
				return
			}

			response, err2 := encodeResponse(message)
			if err2 != nil {
				Logger.Error(err2.Error())
			} else {
				w.Write(response)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			controller.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := controller.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func encodeResponse(message []string) ([]byte, error) {
	toEncode := make(map[string]interface{})

	if len(message) > 0 {
		toEncode["command"] = message[0]
		switch message[0] {
		case SLIDE_SHOW_FINISHED:
		case SLIDE_SHOW_STARTED:
			totalSlides, _ := strconv.Atoi(message[1])
			currentSlide, _ := strconv.Atoi(message[2])
			toEncode["totalSlides"] = totalSlides
			toEncode["currentSlide"] = currentSlide
			toEncode["preview"] = strings.Join([]string{"data:image/png;base64,", message[3]}, "")
		case SLIDE_UPDATED:
			currentSlide, _ := strconv.Atoi(message[1])
			toEncode["currentSlide"] = currentSlide
			toEncode["preview"] = strings.Join([]string{"data:image/png;base64,", message[2]}, "")
		default:
			return nil, errors.New("Failed to encode command")
		}
	}

	encoded, _ := json.Marshal(toEncode)
	return encoded, nil
}
