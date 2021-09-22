package resolvers

import (
	"context"
	"encoding/json"
	"io/ioutil"

	"net/http"

	"github.com/go-fed/activity/streams"
	"github.com/go-fed/activity/streams/vocab"
	"github.com/owncast/owncast/activitypub/apmodels"
	"github.com/owncast/owncast/activitypub/crypto"
	"github.com/owncast/owncast/core/data"

	log "github.com/sirupsen/logrus"
)

// Resolve will translate a raw ActivityPub payload and fire the callback associated with that activity type.
func Resolve(c context.Context, data []byte, callbacks ...interface{}) error {
	jsonResolver, err := streams.NewJSONResolver(callbacks...)
	if err != nil {
		// Something in the setup was wrong. For example, a callback has an
		// unsupported signature and would never be called
		return err
	}

	var jsonMap map[string]interface{}
	if err = json.Unmarshal(data, &jsonMap); err != nil {
		return err
	}

	log.Debugln("Resolving payload...", string(data))

	// The createCallback function will be called.
	err = jsonResolver.Resolve(c, jsonMap)
	if err != nil && !streams.IsUnmatchedErr(err) {
		// Something went wrong
		return err
	} else if streams.IsUnmatchedErr(err) {
		// Everything went right but the callback didn't match or the ActivityStreams
		// type is one that wasn't code generated.
		log.Debugln("No match: ", err)
	}

	return nil
}

// ResolveIRI will resolve an IRI ahd call the correct callback for the resolved type.
func ResolveIRI(c context.Context, iri string, callbacks ...interface{}) error {
	log.Debugln("Resolving", iri)

	req, _ := http.NewRequest("GET", iri, nil)

	actor := apmodels.MakeLocalIRIForAccount(data.GetDefaultFederationUsername())
	if err := crypto.SignRequest(req, nil, actor); err != nil {
		return err
	}

	response, err := http.DefaultClient.Do(req)

	if err != nil {
		return err
	}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	// fmt.Println(string(data))
	return Resolve(c, data, callbacks...)
}

// GetResolvedPersonFromActor resolve a provied actor property to a fully populated person.
func GetResolvedPersonFromActor(actor vocab.ActivityStreamsActorProperty) (vocab.ActivityStreamsPerson, error) {
	var err error
	var person vocab.ActivityStreamsPerson

	personCallback := func(c context.Context, p vocab.ActivityStreamsPerson) error {
		person = p
		return nil
	}

	for iter := actor.Begin(); iter != actor.End(); iter = iter.Next() {
		if iter.IsIRI() {
			iri := iter.GetIRI()
			c := context.TODO()
			if e := ResolveIRI(c, iri.String(), personCallback); e != nil {
				err = e
			}
		} else if iter.IsActivityStreamsPerson() {
			p := iter.GetActivityStreamsPerson()
			person = p
		}
	}

	return person, err
}