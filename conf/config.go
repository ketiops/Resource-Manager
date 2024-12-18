package conf

type GPUNodeAddr struct {
	IPAddr   string
	Password string
	NodeName string
}

type PodCreationRequest struct {
	PodName string `json:"name"`
	Image   string `json:"image"`
	VRAMReq int    `json:"vram"`
}
