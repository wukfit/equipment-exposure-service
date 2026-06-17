package seed

import (
	"github.com/google/uuid"

	"github.com/wukfit/equipment-exposure-service/internal/domain"
)

var (
	AirCatID = uuid.MustParse("2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49")
	JCBID    = uuid.MustParse("36603447-2f30-41b1-a908-526c0b6f1755")
	BobbyID  = uuid.MustParse("713be58e-0d79-4df2-a85c-9f44ca513a7d")
	AliceID  = uuid.MustParse("a52a3c1e-7b2d-4c9a-9f0e-1d6b8c4f2a10")
)

func Equipment() []*domain.EquipmentItem {
	a, _ := domain.NewEquipmentItem(AirCatID, "AirCat - Drill - 4337", 2.1)
	j, _ := domain.NewEquipmentItem(JCBID, "JCB - Hydraulic Breaker - CEJCBHM25", 4.0)
	return []*domain.EquipmentItem{a, j}
}

func Users() []*domain.User {
	b, _ := domain.NewUser(BobbyID, "Bobby Tables")
	al, _ := domain.NewUser(AliceID, "Alice Stone")
	return []*domain.User{b, al}
}
