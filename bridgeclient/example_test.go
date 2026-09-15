package bridgeclient_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/bridgeclient"
)

type examplePluginAPI struct{}

func (examplePluginAPI) PluginHTTP(r *http.Request) *http.Response {
	rec := httptest.NewRecorder()

	switch r.URL.Path {
	case "/" + bridgeclient.PluginID + "/bridge/v1/link":
		replyJSON(rec, http.StatusOK, bridgeclient.LinkResponse{
			Markdown: "[PHIK](/plugins/com.mattermost.plugin-tactical-fusion/decorate/airport?v=PHIK)",
			URL:      "/plugins/com.mattermost.plugin-tactical-fusion/decorate/airport?v=PHIK",
			Type:     bridgeclient.TypeAirport,
			Label:    "PHIK",
		})
	case "/" + bridgeclient.PluginID + "/bridge/v1/decorate":
		replyJSON(rec, http.StatusOK, bridgeclient.DecorateResponse{
			Message:  "Convoy departs [141200ZSEP26](/plugins/com.mattermost.plugin-tactical-fusion/decorate/dtg?a=&dtg=141200ZSEP26&t=1789387200000&z=Z)",
			Changed:  true,
			FitsPost: true,
		})
	default:
		http.NotFound(rec, r)
	}

	return rec.Result()
}

func ExampleClient_Link() {
	client := bridgeclient.NewClient(examplePluginAPI{})

	link, err := client.Link(context.Background(), bridgeclient.LinkRequest{
		Type:  bridgeclient.TypeAirport,
		Token: "PHIK",
	})
	if err != nil {
		fmt.Println("no link:", err)
		return
	}

	fmt.Println("Recovery field " + link.Markdown)
	// Output: Recovery field [PHIK](/plugins/com.mattermost.plugin-tactical-fusion/decorate/airport?v=PHIK)
}

func ExampleClient_Decorate() {
	client := bridgeclient.NewClient(examplePluginAPI{})

	decorated, err := client.Decorate(context.Background(), bridgeclient.DecorateRequest{
		Message: "Convoy departs 141200ZSEP26",
	})
	if err != nil {
		fmt.Println("undecorated:", err)
		return
	}

	fmt.Println(decorated.Changed, decorated.FitsPost)
	fmt.Println(decorated.Message)
	// Output:
	// true true
	// Convoy departs [141200ZSEP26](/plugins/com.mattermost.plugin-tactical-fusion/decorate/dtg?a=&dtg=141200ZSEP26&t=1789387200000&z=Z)
}

func ExampleError() {
	client := bridgeclient.NewClient(examplePluginAPI{})

	_, err := client.Info(context.Background())

	switch {
	case errors.Is(err, bridgeclient.ErrPluginNotActive):
		fmt.Println("Tactical Fusion is not answering; post the plain token instead")
	case errors.Is(err, bridgeclient.ErrDisabled), errors.Is(err, bridgeclient.ErrNotRecognized):
		fmt.Println("leave the token as plain text")
	case err != nil:
		fmt.Println("unexpected:", err)
	}
	// Output: Tactical Fusion is not answering; post the plain token instead
}
