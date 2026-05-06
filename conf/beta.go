package conf

func GetBetaJwtCertName() string {
	return GetConfigString("betaJwtCertName")
}

func GetBetaCodeExpiryDays() int {
	n, _ := GetConfigInt64("betaCodeExpiryDays")
	if n <= 0 {
		return 30
	}
	return int(n)
}

func GetBetaJwtExpiryHours() int {
	n, _ := GetConfigInt64("betaJwtExpiryHours")
	if n <= 0 {
		return 2160
	}
	return int(n)
}

func GetBetaActivateRateLimitPerMinute() int {
	n, _ := GetConfigInt64("betaActivateRateLimitPerMinute")
	if n <= 0 {
		return 10
	}
	return int(n)
}
