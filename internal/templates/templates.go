package templates

import (
	"html/template"
	"io"
)

const authSuccessTemplateContent = `<!DOCTYPE html>
<html>
<head>
    <title>Gust Authentication Success</title>
    <style>
        body {
            font-family: system-ui, sans-serif;
            max-width: 600px;
            margin: 0 auto;
            padding: 2rem;
            text-align: center;
            line-height: 1.6;
        }
        h1 {
            color: #333;
            margin-bottom: 1.5rem;
        }
        .success {
            color: #4CAF50;
            font-weight: bold;
            font-size: 1.2rem;
        }
        .info {
            margin: 2rem 0;
            line-height: 1.5;
        }
        .api-key {
            background: #f5f5f5;
            padding: 1rem;
            border-radius: 4px;
            font-family: monospace;
            overflow-wrap: break-word;
            margin: 1.5rem 0;
            text-align: center;
            border: 1px solid #ddd;
        }
        .next-steps {
            background: #f8f9fa;
            padding: 1rem;
            border-radius: 4px;
            margin-top: 2rem;
        }
        .command {
            background: #2d2d2d;
            color: #fff;
            padding: 0.5rem 1rem;
            border-radius: 4px;
            font-family: monospace;
            display: inline-block;
            margin: 0.5rem 0;
        }
    </style>
</head>
<body>
    <h1>Authentication Successful!</h1>
    <p class="success">Welcome, {{.Login}}!</p>
    <p class="info">Your Gust API key has been generated. Use this API key in your CLI application:</p>
    <div class="api-key">{{.ApiKey}}</div>
    <p>This key will allow you to access weather data through the Gust API.</p>

    <div class="next-steps">
        <p>You can now return to your terminal. The CLI application should automatically continue.</p>
        <p>If it doesn't, you can close this window and manually run the CLI with your API key.</p>
    </div>
</body>
</html>`

var templates *template.Template

func init() {
	templates = template.Must(template.New("auth_success").Parse(authSuccessTemplateContent))
}

func RenderSuccessTemplate(w io.Writer, login, apiKey string) error {
	data := struct {
		Login  string
		ApiKey string
	}{
		Login:  login,
		ApiKey: apiKey,
	}

	return templates.ExecuteTemplate(w, "auth_success", data)
}
