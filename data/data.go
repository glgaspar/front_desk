package data

import (
	"encoding/json"
	"front_desk/models"
	"os"
)

func GetPayChecker() (*[]models.PayCheckerBill, error ){
	response, err := Api("GET", nil, nil, os.Getenv("PAYCHECKER_HOST"))
	if err != nil {
		return nil, err
	}

	var result models.ApiResult
	if err = json.Unmarshal(*response, &result); err != nil {
		return nil, err
	}

	var data []models.PayCheckerBill
	if err = json.Unmarshal(result.Data, &data); err != nil {
		return nil, err
	}

	return &data, nil
}