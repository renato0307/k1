package commands

import "strings"

// MockLLMTranslation represents a simulated LLM translation result
type MockLLMTranslation struct {
	Prompt      string // Original natural language prompt
	Command     string // Generated kubectl command
	Explanation string // Brief explanation
}

// TranslateWithMockLLM simulates LLM translation using static mappings
func TranslateWithMockLLM(prompt string) MockLLMTranslation {
	lowerPrompt := strings.ToLower(strings.TrimSpace(prompt))

	// Static mock responses
	mockResponses := map[string]MockLLMTranslation{
		"delete failing pods": {
			Prompt:      prompt,
			Command:     "kubectl delete pods --field-selector status.phase=Failed",
			Explanation: "Deletes all pods in Failed status",
		},
		"scale nginx to 3": {
			Prompt:      prompt,
			Command:     "kubectl scale deployment nginx --replicas=3",
			Explanation: "Scales nginx deployment to 3 replicas",
		},
		"get pod logs": {
			Prompt:      prompt,
			Command:     "kubectl logs <selected-pod>",
			Explanation: "Shows logs for the selected pod",
		},
		"restart deployment": {
			Prompt:      prompt,
			Command:     "kubectl rollout restart deployment <selected>",
			Explanation: "Restarts the selected deployment",
		},
		"show pod events": {
			Prompt:      prompt,
			Command:     "kubectl get events --field-selector involvedObject.name=<selected>",
			Explanation: "Shows events for the selected pod",
		},
	}

	// Look for exact match first
	if translation, ok := mockResponses[lowerPrompt]; ok {
		return translation
	}

	// Look for partial matches
	for key, translation := range mockResponses {
		if strings.Contains(lowerPrompt, key) || strings.Contains(key, lowerPrompt) {
			return translation
		}
	}

	// Default fallback
	return MockLLMTranslation{
		Prompt:      prompt,
		Command:     "kubectl get pods",
		Explanation: "No specific translation found - showing default command",
	}
}
