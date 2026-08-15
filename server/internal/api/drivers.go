package api

import (
	"net/http"

	"xmedia/internal/driver"
)

func (h *Handler) listDrivers(w http.ResponseWriter, _ *http.Request) {
	writeOK(w, driver.List())
}
