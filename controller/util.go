package controller

import (
	betterguid "github.com/kjk/betterguid"
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
