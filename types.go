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
	Channel  string                 `json:"channel"`
	Text     string                 `json:"text"`
	TTL      int                    `json:"ttl"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
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

type ReactionAddedEvent struct {
	Token   string `json:"token"`
	Type    string `json:"type"`
	EventID string `json:"event_id"`
	Event   struct {
		Type     string `json:"type"`
		User     string `json:"user"`
		Reaction string `json:"reaction"`
		Item     struct {
			Type    string `json:"type"`
			Channel string `json:"channel"`
			Ts      string `json:"ts"`
		} `json:"item"`
		ItemUser string `json:"item_user"`
		EventTs  string `json:"event_ts"`
	} `json:"event"`
	Authorizations []struct {
		UserID string `json:"user_id"`
		IsBot  bool   `json:"is_bot"`
	} `json:"authorizations"`
}

type MessageMetadata struct {
	EventType    string                 `json:"event_type"`
	EventPayload map[string]interface{} `json:"event_payload"`
}

type GitHubWebhookEvent struct {
	Action string `json:"action"`
	Issue  struct {
		URL           string `json:"url"`
		RepositoryURL string `json:"repository_url"`
		Number        int    `json:"number"`
		Title         string `json:"title"`
	} `json:"issue"`
}

type SlackReaction struct {
	Reaction string `json:"reaction"`
	Channel  string `json:"channel"`
	Ts       string `json:"ts"`
}

type TimeBombMessage struct {
	Channel string `json:"channel"`
	Ts      string `json:"ts"`
	TTL     int    `json:"ttl"`
}

type MessageActionEvent struct {
	Type       string `json:"type"`
	Token      string `json:"token"`
	ActionTs   string `json:"action_ts"`
	CallbackID string `json:"callback_id"`
	TriggerID  string `json:"trigger_id"`
	MessageTs  string `json:"message_ts"`
	User       struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"user"`
	Channel struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"channel"`
	Message struct {
		Type string `json:"type"`
		User string `json:"user"`
		Ts   string `json:"ts"`
		Text string `json:"text"`
	} `json:"message"`
}

type TitleGenerationOutput struct {
	Version int    `json:"version"`
	Title   string `json:"title"`
	Prompt  string `json:"prompt"`
}
