package sni

type EventDefinition struct {
    Name   string `json:"name"`
    Address uint32 `json:"address"`
    Value  []byte `json:"value"`
    Action string `json:"action"`
}

type DetectedEvent struct {
    Action string
}

func matches(data, expected []byte) bool {
    if len(data) != len(expected) {
        return false
    }
    for i, v := range data {
        if v != expected[i] {
            return false
        }
    }
    return true
}
