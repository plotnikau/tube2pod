package app

import (
	"testing"
)

func TestGetAudioFilename(t *testing.T) {
	id := "test-id"
	expected := TmpDir + id + ExtAudio
	actual := GetAudioFilename(id)
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestGetAudioFilenamePattern(t *testing.T) {
	id := "test-id"
	expected := TmpDir + id + ".%02d" + ExtAudio
	actual := GetAudioFilenamePattern(id)
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestGetVideoFilename(t *testing.T) {
	id := "test-id"
	expected := TmpDir + id + ExtVideo
	actual := GetVideoFilename(id)
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}
