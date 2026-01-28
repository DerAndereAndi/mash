package log

import "testing"

func TestDirectionString(t *testing.T) {
	tests := []struct {
		dir  Direction
		want string
	}{
		{DirectionIn, "IN"},
		{DirectionOut, "OUT"},
		{Direction(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.dir.String()
		if got != tt.want {
			t.Errorf("Direction(%d).String() = %q, want %q", tt.dir, got, tt.want)
		}
	}
}

func TestLayerString(t *testing.T) {
	tests := []struct {
		layer Layer
		want  string
	}{
		{LayerTransport, "TRANSPORT"},
		{LayerWire, "WIRE"},
		{LayerService, "SERVICE"},
		{Layer(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.layer.String()
		if got != tt.want {
			t.Errorf("Layer(%d).String() = %q, want %q", tt.layer, got, tt.want)
		}
	}
}

func TestCategoryString(t *testing.T) {
	tests := []struct {
		cat  Category
		want string
	}{
		{CategoryMessage, "MESSAGE"},
		{CategoryControl, "CONTROL"},
		{CategoryState, "STATE"},
		{CategoryError, "ERROR"},
		{Category(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.cat.String()
		if got != tt.want {
			t.Errorf("Category(%d).String() = %q, want %q", tt.cat, got, tt.want)
		}
	}
}

func TestRoleString(t *testing.T) {
	tests := []struct {
		role Role
		want string
	}{
		{RoleDevice, "DEVICE"},
		{RoleController, "CONTROLLER"},
		{Role(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.role.String()
		if got != tt.want {
			t.Errorf("Role(%d).String() = %q, want %q", tt.role, got, tt.want)
		}
	}
}

func TestMessageTypeString(t *testing.T) {
	tests := []struct {
		mt   MessageType
		want string
	}{
		{MessageTypeRequest, "REQUEST"},
		{MessageTypeResponse, "RESPONSE"},
		{MessageTypeNotification, "NOTIFICATION"},
		{MessageType(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.mt.String()
		if got != tt.want {
			t.Errorf("MessageType(%d).String() = %q, want %q", tt.mt, got, tt.want)
		}
	}
}

func TestStateEntityString(t *testing.T) {
	tests := []struct {
		entity StateEntity
		want   string
	}{
		{StateEntityConnection, "CONNECTION"},
		{StateEntitySession, "SESSION"},
		{StateEntityCommissioning, "COMMISSIONING"},
		{StateEntity(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.entity.String()
		if got != tt.want {
			t.Errorf("StateEntity(%d).String() = %q, want %q", tt.entity, got, tt.want)
		}
	}
}

func TestControlMsgTypeString(t *testing.T) {
	tests := []struct {
		cmt  ControlMsgType
		want string
	}{
		{ControlMsgPing, "PING"},
		{ControlMsgPong, "PONG"},
		{ControlMsgClose, "CLOSE"},
		{ControlMsgType(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.cmt.String()
		if got != tt.want {
			t.Errorf("ControlMsgType(%d).String() = %q, want %q", tt.cmt, got, tt.want)
		}
	}
}

func TestDirectionValues(t *testing.T) {
	// Verify explicit values for wire stability
	if DirectionIn != 0 {
		t.Errorf("DirectionIn = %d, want 0", DirectionIn)
	}
	if DirectionOut != 1 {
		t.Errorf("DirectionOut = %d, want 1", DirectionOut)
	}
}

func TestLayerValues(t *testing.T) {
	// Verify explicit values for wire stability
	if LayerTransport != 0 {
		t.Errorf("LayerTransport = %d, want 0", LayerTransport)
	}
	if LayerWire != 1 {
		t.Errorf("LayerWire = %d, want 1", LayerWire)
	}
	if LayerService != 2 {
		t.Errorf("LayerService = %d, want 2", LayerService)
	}
}

func TestCategoryValues(t *testing.T) {
	// Verify explicit values for wire stability
	if CategoryMessage != 0 {
		t.Errorf("CategoryMessage = %d, want 0", CategoryMessage)
	}
	if CategoryControl != 1 {
		t.Errorf("CategoryControl = %d, want 1", CategoryControl)
	}
	if CategoryState != 2 {
		t.Errorf("CategoryState = %d, want 2", CategoryState)
	}
	if CategoryError != 3 {
		t.Errorf("CategoryError = %d, want 3", CategoryError)
	}
}

func TestRoleValues(t *testing.T) {
	// Verify explicit values for wire stability
	if RoleDevice != 0 {
		t.Errorf("RoleDevice = %d, want 0", RoleDevice)
	}
	if RoleController != 1 {
		t.Errorf("RoleController = %d, want 1", RoleController)
	}
}

func TestMessageTypeValues(t *testing.T) {
	// Verify explicit values for wire stability
	if MessageTypeRequest != 0 {
		t.Errorf("MessageTypeRequest = %d, want 0", MessageTypeRequest)
	}
	if MessageTypeResponse != 1 {
		t.Errorf("MessageTypeResponse = %d, want 1", MessageTypeResponse)
	}
	if MessageTypeNotification != 2 {
		t.Errorf("MessageTypeNotification = %d, want 2", MessageTypeNotification)
	}
}

func TestStateEntityValues(t *testing.T) {
	// Verify explicit values for wire stability
	if StateEntityConnection != 0 {
		t.Errorf("StateEntityConnection = %d, want 0", StateEntityConnection)
	}
	if StateEntitySession != 1 {
		t.Errorf("StateEntitySession = %d, want 1", StateEntitySession)
	}
	if StateEntityCommissioning != 2 {
		t.Errorf("StateEntityCommissioning = %d, want 2", StateEntityCommissioning)
	}
}

func TestControlMsgTypeValues(t *testing.T) {
	// Verify explicit values for wire stability
	if ControlMsgPing != 0 {
		t.Errorf("ControlMsgPing = %d, want 0", ControlMsgPing)
	}
	if ControlMsgPong != 1 {
		t.Errorf("ControlMsgPong = %d, want 1", ControlMsgPong)
	}
	if ControlMsgClose != 2 {
		t.Errorf("ControlMsgClose = %d, want 2", ControlMsgClose)
	}
}
