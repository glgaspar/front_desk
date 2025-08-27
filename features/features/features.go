package features

type Feature struct {
	Img  string `json:"Img"`
	Name string `json:"Name"`
	Path string `json:"Path"`
}

func (f *Feature) Show() []Feature {
	featureList := []Feature{
					{
						Img: "",
						Name: "PayChecker",
						Path: "/paychecker",
					},
					{
						Img: "",
						Name: "TimeTracker",
						Path: "/timetracker",
					},
				}

	return featureList
}


