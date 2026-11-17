package main

import (
	"net/http"

	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi31"
)

// stopsRequest exists only to describe GET /stops: handleStops reads the query by hand, because the
// params repeat and pair up by position. Keep it in step with handleStops (TestOpenAPI checks the names).
type stopsRequest struct {
	Title   string   `query:"title" description:"Trip title, echoed back. Blank gives אוטובוסים." example:"מהבית לעבודה"`
	Stop    []string `query:"stop" description:"Board at this stop (makat, 1-6 digits). Repeats, up to 4; the Nth of each repeating param describes option N. Blank with a shuttle makes a shuttle-only option."`
	To      []string `query:"to" description:"Get off at this stop (digits). Keeps only lines that reach it after stop, which picks the direction."`
	Lines   []string `query:"lines" description:"Comma-separated line numbers, matched exactly (1 is not 1א). Blank means all."`
	Label   []string `query:"label" description:"Header text. Blank uses the stop name."`
	Walk    []string `query:"walk" description:"Minutes from the door to the stop, 0-60. Times come back as leave times (bus time minus walk); buses you can no longer reach are dropped."`
	Shuttle []string `query:"shuttle" description:"Hand-entered departures, HH:MM separated by comma, semicolon or whitespace. Today only, Israel time; merged in as line שאטל."`
}

// openAPISpec builds the spec from the same Go types the handler fills in, so the schema cannot drift from the JSON.
func openAPISpec() ([]byte, error) {
	r := openapi31.NewReflector()
	r.SpecEns().WithOpenapi("3.1.0").Info.WithTitle("israel-bus").WithVersion("1").
		WithDescription("Upcoming Israeli bus and light rail departures for a TRMNL display. Data only; the plugin's Liquid draws it. Source: Ministry of Transport API behind route.bus.gov.il.")
	op, err := r.NewOperationContext(http.MethodGet, "/stops")
	if err != nil {
		return nil, err
	}
	op.SetSummary("Departures for a trip")
	op.AddReqStructure(new(stopsRequest))
	op.AddRespStructure(new(Response), openapi.WithHTTPStatus(http.StatusOK))
	op.AddRespStructure(new(string), openapi.WithHTTPStatus(http.StatusBadRequest), openapi.WithContentType("text/plain"))
	if err := r.AddOperation(op); err != nil {
		return nil, err
	}
	return r.Spec.MarshalYAML()
}

func handleOpenAPI(w http.ResponseWriter, _ *http.Request) {
	b, err := openAPISpec()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.Write(b)
}
