package calculator

import (
	"context"
	"testing"
)

func TestExecute(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
		want      string
	}{
		{
			name:      "加法",
			arguments: `{"num1":10,"num2":5,"operation":"add"}`,
			want:      "15",
		},
		{
			name:      "减法",
			arguments: `{"num1":10,"num2":5,"operation":"subtract"}`,
			want:      "5",
		},
		{
			name:      "乘法",
			arguments: `{"num1":10,"num2":5,"operation":"multiply"}`,
			want:      "50",
		},
		{
			name:      "除法",
			arguments: `{"num1":10,"num2":4,"operation":"divide"}`,
			want:      "2.5",
		},
	}

	tool := New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tool.Execute(context.Background(), tt.arguments)
			if err != nil {
				t.Fatal(err)
			}

			if got != tt.want {
				t.Fatalf("unexpected result: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExecuteErrors(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
	}{
		{
			name:      "JSON格式错误",
			arguments: `{`,
		},
		{
			name:      "缺少运算类型",
			arguments: `{"num1":10,"num2":5}`,
		},
		{
			name:      "除数为零",
			arguments: `{"num1":10,"num2":0,"operation":"divide"}`,
		},
		{
			name:      "不支持的运算",
			arguments: `{"num1":10,"num2":5,"operation":"mod"}`,
		},
	}

	tool := New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(
				context.Background(),
				tt.arguments,
			)

			if err == nil {
				t.Fatalf("expected an error, got result %q", result)
			}
		})
	}
}
