package push

// Keys mirrors the PushSubscriptionJSON.keys shape the browser's
// PushManager.subscribe() returns — p256dh/auth are the encryption keys
// this server needs to address an encrypted push at that specific
// subscription.
type Keys struct {
	P256dh string `json:"p256dh"`
	Auth   string `json:"auth"`
}

type subscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     Keys   `json:"keys"`
}

type unsubscribeRequest struct {
	Endpoint string `json:"endpoint"`
}

type vapidKeyResponse struct {
	PublicKey string `json:"publicKey"`
}
