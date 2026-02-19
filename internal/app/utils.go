package app

func GetAudioFilename(id string) string {
	return TmpDir + id + ExtAudio
}

func GetAudioFilenamePattern(id string) string {
	return TmpDir + id + ".%02d" + ExtAudio
}

func GetVideoFilename(id string) string {
	return TmpDir + id + ExtVideo
}

func GetThumbnailFilename(id string) string {
	return TmpDir + id + ExtThumb
}
