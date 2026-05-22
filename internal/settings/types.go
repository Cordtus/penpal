package settings

type (
	Config struct {
		Networks  []Network `json:"networks"`
		Notifiers Notifiers `json:"notifiers"`
		Health    Health    `json:"health"`
	}

	Network struct {
		Name           string   `json:"name"`
		ChainId        string   `json:"chain_id"`
		Address        string   `json:"address"`
		Rpcs           []string `json:"rpcs"`
		RpcAlert       bool     `json:"rpc_alert"`
		SignerMetrics  string   `json:"signer_metrics"`
		SignerStallMins int     `json:"signer_stall_mins"`
		BackCheck      int      `json:"back_check"`
		AlertThreshold int      `json:"alert_threshold"`
		Interval       int      `json:"interval"`
		StallTime      int      `json:"stall_time"`
	}

	Health struct {
		Interval int      `json:"interval"`
		Port     string   `json:"port"`
		Nodes    []string `json:"nodes"`
	}

	Notifiers struct {
		Telegram struct {
			Key  string `json:"key"`
			Chat string `json:"chat_id"`
		} `json:"telegram"`
		Discord struct {
			Webhook string `json:"webhook"`
		} `json:"discord"`
	}
)
