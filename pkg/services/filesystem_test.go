package services

import (
	"testing"

	"ctfd-downloader/pkg/models"
)

func TestHasLaunchableInstance(t *testing.T) {
	docker := &models.ChallengeDetailed{}
	docker.Type = "dynamic_docker"
	if !hasLaunchableInstance(docker) {
		t.Error("docker type should be a launchable instance")
	}

	whale := &models.ChallengeDetailed{View: `<div id="whale-panel">Launch an instance</div>`}
	if !hasLaunchableInstance(whale) {
		t.Error("whale-panel view should be a launchable instance")
	}

	plain := &models.ChallengeDetailed{View: "<div>just a description</div>"}
	plain.Type = "standard"
	if hasLaunchableInstance(plain) {
		t.Error("standard challenge should not be an instance")
	}
}
