package config

import "testing"

func TestInferCompatKindFromBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "minimax cn anthropic",
			baseURL: "https://api.minimaxi.com/anthropic",
			want:    "minimax",
		},
		{
			name:    "minimax global anthropic",
			baseURL: "https://api.minimaxi.io/anthropic",
			want:    "minimax",
		},
		{
			name:    "minimax trailing slash",
			baseURL: "https://api.minimaxi.com/anthropic/",
			want:    "minimax",
		},
		{
			name:    "other provider",
			baseURL: "https://api.anthropic.com",
			want:    "",
		},
		{
			name:    "kimi coding",
			baseURL: "https://api.kimi.com/coding",
			want:    "kimi",
		},
		{
			name:    "zhipu anthropic",
			baseURL: "https://open.bigmodel.cn/api/anthropic",
			want:    "zhipu",
		},
		{
			name:    "lanyun anthropic",
			baseURL: "https://maas-api.lanyun.net/anthropic",
			want:    "zhipu",
		},
		{
			name:    "xfyun anthropic",
			baseURL: "https://maas-coding-api.cn-huabei-1.xf-yun.com/anthropic",
			want:    "xfyun",
		},
		{
			name:    "xiaomi anthropic",
			baseURL: "https://token-plan-cn.xiaomimimo.com/anthropic",
			want:    "xiaomi",
		},
		{
			name:    "qwen anthropic app",
			baseURL: "https://coding.dashscope.aliyuncs.com/apps/anthropic",
			want:    "qwen",
		},
		{
			name:    "doubao coding",
			baseURL: "https://ark.cn-beijing.volces.com/api/coding",
			want:    "doubao",
		},
		{
			name:    "minimax non anthropic path",
			baseURL: "https://api.minimaxi.com/v1",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InferCompatKindFromBaseURL(tt.baseURL); got != tt.want {
				t.Fatalf("InferCompatKindFromBaseURL(%q) = %q, want %q", tt.baseURL, got, tt.want)
			}
		})
	}
}
