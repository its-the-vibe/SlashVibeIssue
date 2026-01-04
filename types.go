package main

type SlackCommand struct {
	Command     string `json:"command"`
	Text        string `json:"text"`
	ResponseURL string `json:"response_url"`
	TriggerID   string `json:"trigger_id"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	ChannelID   string `json:"channel_id"`
}

type ViewSubmission struct {
	Type string `json:"type"`
	View struct {
		CallbackID string `json:"callback_id"`
		State      struct {
			Values map[string]map[string]interface{} `json:"values"`
		} `json:"state"`
	} `json:"view"`
	User struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"user"`
}

type SlackLinerMessage struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
	TTL     int    `json:"ttl"`
}

type PoppitCommand struct {
	Repo     string                 `json:"repo"`
	Branch   string                 `json:"branch"`
	Type     string                 `json:"type"`
	Dir      string                 `json:"dir"`
	Commands []string               `json:"commands"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type PoppitOutput struct {
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Type     string                 `json:"type"`
	Command  string                 `json:"command"`
	Output   string                 `json:"output"`
}
