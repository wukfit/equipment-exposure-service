package seed

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEquipmentFixtures(t *testing.T) {
	items := Equipment()
	require.Len(t, items, 2)

	var aircat, jcb bool
	for _, it := range items {
		switch it.ID() {
		case AirCatID:
			assert.Equal(t, "AirCat - Drill - 4337", it.Name())
			assert.Equal(t, 2.1, it.VibrationMagnitude())
			aircat = true
		case JCBID:
			assert.Equal(t, "JCB - Hydraulic Breaker - CEJCBHM25", it.Name())
			assert.Equal(t, 4.0, it.VibrationMagnitude())
			jcb = true
		}
	}
	assert.True(t, aircat, "AirCat fixture missing")
	assert.True(t, jcb, "JCB fixture missing")
}

func TestUserFixtures(t *testing.T) {
	users := Users()
	require.Len(t, users, 2)

	var bobby, alice bool
	for _, u := range users {
		switch u.ID() {
		case BobbyID:
			assert.Equal(t, "Bobby Tables", u.Name())
			bobby = true
		case AliceID:
			assert.Equal(t, "Alice Stone", u.Name())
			alice = true
		}
	}
	assert.True(t, bobby, "Bobby Tables fixture missing")
	assert.True(t, alice, "Alice Stone fixture missing")
}
