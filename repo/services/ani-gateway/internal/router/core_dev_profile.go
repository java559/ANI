package router

type coreDevProfileResponse struct {
	Mode         string `json:"mode"`
	Provider     string `json:"provider"`
	RealProvider bool   `json:"real_provider"`
	Reason       string `json:"reason"`
}

func localCoreDevProfile(provider string, reason string) coreDevProfileResponse {
	return coreDevProfileResponse{
		Mode:         "local",
		Provider:     provider,
		RealProvider: false,
		Reason:       reason,
	}
}
