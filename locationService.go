package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

//Structure to hold Json object fetched from GoogleMapApi
type MyJson struct {
	Results []struct {
		AddressComponents []struct {
			LongName  string   `json:"long_name"`
			ShortName string   `json:"short_name"`
			Types     []string `json:"types"`
		} `json:"address_components"`
		FormattedAddress string `json:"formatted_address"`
		Geometry         struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
			LocationType string `json:"location_type"`
			Viewport     struct {
				Northeast struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"northeast"`
				Southwest struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"southwest"`
			} `json:"viewport"`
		} `json:"geometry"`
		PlaceID string   `json:"place_id"`
		Types   []string `json:"types"`
	} `json:"results"`
	Status string `json:"status"`
}

//Structure to hold Json object for storing locatiosn
type Location struct {
	Id         bson.ObjectId `json:"id" bson:"_id"`
	Name       string        `json:"name"`
	Address    string        `json:"address"`
	City       string        `json:"city"`
	State      string        `json:"state"`
	Zip        string        `json:"zip"`
	Coordinate struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"coordinate"`
}

type LocationSession struct {
	session *mgo.Session
}

// NewLocationSession provides a reference to a LocationSession with provided mongo session
func NewLocationSession(s *mgo.Session) *LocationSession {
	return &LocationSession{s}
}

//This function call GoogleMapApi and fetches location coordinates for given address
func getcoordinates(l Location) (MyJson, error) {

	//Adding complete address in one string
	addr := "" + l.Address + " " + l.City + " " + l.State

	str := strings.Split(addr, " ")

	s := ""

	//Making address that can be used with url for GoogleMapApi
	for _, key := range str {
		s += (key + "+")
	}

	s = s[:len(s)-1]

	//Making url to fetch details of location form GoogleMapApi
	url := "http://maps.google.com/maps/api/geocode/json?address=" + s + "&sensor=false"
	res, err := http.Get(url)

	var f MyJson

	if err != nil {
		return f, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return f, err
	}

	err = json.Unmarshal(body, &f)
	if err != nil {
		return f, err
	}

	return f, nil

}

// CreateLocation creates a new user resource
func (ls LocationSession) CreateLocation(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// Stub location to be populated from the body
	u := Location{}

	// Populate the location data
	json.NewDecoder(r.Body).Decode(&u)

	// Add an Id
	u.Id = bson.NewObjectId()

	//Calling getcoordinates() to fecth location coordinates from GoogleMapApi
	var c MyJson
	var err error

	c, err = getcoordinates(u)

	if err != nil || c.Status != "OK" {
		//fmt.Println("Error in address provided or Json request")
		w.WriteHeader(400)
		return
	}

	//Adding coordinate for Location
	u.Coordinate = c.Results[0].Geometry.Location

	// Write the user to mongo
	ls.session.DB("locations").C("places").Insert(u)

	// Marshal provided interface into JSON structure
	uj, _ := json.Marshal(u)

	// Write content-type, statuscode, payload
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	fmt.Fprintf(w, "%s", uj)
}

// ReadLocation retrieves an individual location resource
func (ls LocationSession) ReadLocation(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	// Grab id
	id := p.ByName("id")

	// Verify id is ObjectId, otherwise bail
	if !bson.IsObjectIdHex(id) {
		w.WriteHeader(404)
		return
	}

	// Grab id
	oid := bson.ObjectIdHex(id)

	// Stub location
	u := Location{}

	// Fetch location
	if err := ls.session.DB("locations").C("places").FindId(oid).One(&u); err != nil {
		w.WriteHeader(404)
		return
	}

	// Marshal provided interface into JSON structure
	uj, _ := json.Marshal(u)

	// Write content-type, statuscode, payload
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	fmt.Fprintf(w, "%s", uj)
}

//UpdateLocation updates an existing location
func (ls LocationSession) UpdateLocation(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	// Stub a location to be populated from the body
	id := p.ByName("id")

	// Verify id is ObjectId, otherwise bail
	if !bson.IsObjectIdHex(id) {
		w.WriteHeader(404)
		return
	}

	pu := Location{}

	//Populate the location data
	json.NewDecoder(r.Body).Decode(&pu)

	// Grab id
	oid := bson.ObjectIdHex(id)

	// Stub location
	u := Location{}

	if err := ls.session.DB("locations").C("places").FindId(oid).One(&u); err != nil {
		w.WriteHeader(404)
		return
	}

	u.Address = pu.Address
	u.City = pu.City
	u.State = pu.State
	u.Zip = pu.Zip

	var f MyJson
	var err error

	f, err = getcoordinates(u)

	if err != nil || f.Status != "OK" {
		//fmt.Println("Error in address provided or Json request")
		w.WriteHeader(400)
		return
	}

	//Adding coordinate for Location
	u.Coordinate = f.Results[0].Geometry.Location

	c := ls.session.DB("locations").C("places")

	err = c.UpdateId(oid, u)

	if err != nil {
		w.WriteHeader(404)
		return
	}

	l := Location{}

	if err := ls.session.DB("locations").C("places").FindId(oid).One(&l); err != nil {
		fmt.Println("error : ", err)
		w.WriteHeader(404)
		return
	}

	// Marshal provided interface into JSON structure
	uj, _ := json.Marshal(l)

	// Write content-type, statuscode, payload
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	fmt.Fprintf(w, "%s", uj)
}

// DeleteLocation removes an existing location resource
func (ls LocationSession) DeleteLocation(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	// Grab id
	id := p.ByName("id")

	// Verify id is ObjectId, otherwise bail
	if !bson.IsObjectIdHex(id) {
		w.WriteHeader(404)
		return
	}

	// Grab id
	oid := bson.ObjectIdHex(id)

	// Remove user
	if err := ls.session.DB("locations").C("places").RemoveId(oid); err != nil {
		w.WriteHeader(404)
		return
	}

	// Write status
	w.WriteHeader(200)
}

//This function creates mongosession or panics if error occurs
func getSession() *mgo.Session {

	// Connect to mongodb
	s, err := mgo.Dial("mongodb://dbuserShubhra:shubhra123@ds045054.mongolab.com:45054/locations")

	// Check if connection error, is mongo running?
	if err != nil {
		panic(err)
	}

	// Deliver session
	return s
}

func main() {
	// Instantiate a new router
	r := httprouter.New()

	//Creating new location session with mgosession
	nls := NewLocationSession(getSession())

	// Add a handler
	r.POST("/locations", nls.CreateLocation)
	r.GET("/locations/:id", nls.ReadLocation)
	r.PUT("/locations/:id", nls.UpdateLocation)
	r.DELETE("/locations/:id", nls.DeleteLocation)

	// Fire up the server
	http.ListenAndServe("localhost:3030", r)
}
