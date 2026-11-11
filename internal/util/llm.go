package util

import (
	"StudioAttendance_backend/internal/model"
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)


func ParseScheduleImage(URL string)([]model.Schedule,error){
	client := openai.NewClient(
		option.WithAPIKey(os.Getenv("DEEPSEEK_API_KEY")),
		option.WithBaseURL("https://api.deepseek.com"),
	)


	prompt := `
        请识别这张大学课表图片，提取所有课程信息。

        严格返回 JSON 数组，不要返回 Markdown 代码块，不要返回任何解释文字。

        每个课程对象的字段名必须与下面完全一致：
        course_name   课程名，字符串
        location      上课地点，字符串
        weekday       周几，整数 1~7（周一=1，周日=7），不要用"周一"这种文字
        start_section 开始节次，整数
        end_section   结束节次，整数
        start_week    起始周，整数
        end_week      结束周，整数
        week_type     周次类型，只能是 "全周"、"单周"、"双周" 之一

        无法识别的字段：字符串填空字符串，整数填 0，week_type 填 "全周"。
    `
	completion , err := client.Chat.Completions.New(
		context.Background(),
		openai.ChatCompletionNewParams{
			Model: openai.ChatModel("deepseek-flash"),
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(
					[]openai.ChatCompletionContentPartUnionParam{
						openai.TextContentPart(prompt),
						openai.ImageContentPart(
							openai.ChatCompletionContentPartImageImageURLParam{
								URL: URL,
							},
						),
					},
				),
			},
		},
	)
	if err != nil{
		return nil,err
	}
	content := strings.TrimSpace(
		completion.Choices[0].Message.Content,
	)
	var schedule []model.Schedule
	if err := json.Unmarshal([]byte(content), &schedule); err != nil {
		return nil, err
	}
	return schedule, nil
}