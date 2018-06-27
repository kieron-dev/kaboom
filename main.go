package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/pivotal-cf/brokerapi"
)

const SERVICE_PREFIX = "svc_"

type registerServiceData struct {
	Name          string `json:"name"`
	HelmChartName string `json:"helm_chart_name"`
}

type kaboomBroker struct {
	redisClient *redis.Client
}

func (k *kaboomBroker) Services(ctx context.Context) (catalog []brokerapi.Service, err error) {
	log.Println("starting catalog request")
	stringSlice := k.redisClient.Keys(fmt.Sprintf("%s*", SERVICE_PREFIX))
	if stringSlice.Err() != nil {
		log.Println("could not fetch keys from redis")
		return nil, stringSlice.Err()
	}
	for _, key := range stringSlice.Val() {
		getStatus := k.redisClient.Get(key)
		if getStatus.Err() != nil {
			log.Printf("could not get redis key: %v", key)
			return nil, err
		}
		v := getStatus.Val()
		s := new(registerServiceData)
		if err := json.Unmarshal([]byte(v), s); err != nil {
			log.Printf("could not unmarshal redis value: %v", v)
			return nil, err
		}
		catalog = append(catalog, brokerapi.Service{
			ID:            s.Name,
			Name:          s.Name,
			Description:   "ye",
			Bindable:      true,
			PlanUpdatable: false,
			Plans: []brokerapi.ServicePlan{
				{
					ID:          fmt.Sprintf("plan-%s", s.Name),
					Name:        "Default",
					Description: "Just the default",
				},
			},
		})
	}
	log.Println("all done - returning")
	return
}

func (k *kaboomBroker) Provision(ctx context.Context, instanceID string, details brokerapi.ProvisionDetails, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, error) {
	getStatus := k.redisClient.Get(fmt.Sprintf("%s%s", SERVICE_PREFIX, details.ServiceID))
	if getStatus.Err() != nil {
		log.Printf("getStatus.Err() = %+v\n", getStatus.Err())
		return brokerapi.ProvisionedServiceSpec{}, getStatus.Err()
	}

	v := getStatus.Val()
	s := new(registerServiceData)
	if err := json.Unmarshal([]byte(v), s); err != nil {
		log.Printf("could not unmarshal redis value: %v", v)
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	name := s.HelmChartName
	output, err := installChart(name)
	if err != nil {
		log.Printf("Error while installing helm chart: %v - %v", err, output)
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	log.Println("ALL GOOD")
	log.Println(output)

	lines := strings.Split(output, "\n")
	words := strings.Split(lines[0], " ")
	release := words[len(words)-1]

	return brokerapi.ProvisionedServiceSpec{
		IsAsync:       true,
		OperationData: fmt.Sprintf("{\"release_name\": \"%s\"}", release),
	}, nil
}

func (k *kaboomBroker) LastOperation(ctx context.Context, instanceID string, operationData string) (brokerapi.LastOperation, error) {
	time.Sleep(time.Second * 2)
	dtls := struct {
		ReleaseName string `json:"release_name"`
	}{}
	err := json.Unmarshal([]byte(operationData), &dtls)
	if err != nil {
		log.Printf("Couldn't unmarshal operation data: %s\n", operationData)
		return brokerapi.LastOperation{}, err
	}
	return brokerapi.LastOperation{
		State:       "succeeded",
		Description: fmt.Sprintf("Successfully deployed release %s\n", dtls.ReleaseName),
	}, nil
}

func (k *kaboomBroker) Deprovision(ctx context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	panic("not implemented")
}

func (k *kaboomBroker) Bind(ctx context.Context, instanceID string, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	panic("not implemented")
}

func (k *kaboomBroker) Unbind(ctx context.Context, instanceID string, bindingID string, details brokerapi.UnbindDetails) error {
	panic("not implemented")
}

func (k *kaboomBroker) Update(ctx context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	panic("not implemented")
}

func installChart(name string) (string, error) {
	helmHost := os.Getenv("HELM_HOST")
	command := exec.Command("helm", "--host", helmHost, "install", name)
	o, err := command.CombinedOutput()
	if err != nil {
		log.Printf("Error installing chart: %s - %v", string(o), err)
		return "", err
	}
	log.Printf("IT WORKS. %v", string(o))
	return string(o), nil
}

func main() {
	redisServer := os.Getenv("REDIS_SERVER")
	redisCStr := fmt.Sprintf("%s:6379", redisServer)
	redisClient := redis.NewClient(&redis.Options{Addr: redisCStr, Password: ""})

	brokerRouter := mux.NewRouter()
	brokerRouter.HandleFunc("/register-service/{name}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		name := vars["name"]
		if name == "" {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "name is empty")
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err.Error())
			return
		}
		defer r.Body.Close()

		j := registerServiceData{}
		if err = json.Unmarshal(body, &j); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err.Error())
			return
		}
		if j.HelmChartName == "" {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "helmChartName is empty")
			return
		}

		j.Name = name
		serviceJson, err := json.Marshal(j)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err.Error())
			return
		}

		status := redisClient.Set(fmt.Sprintf("%s%s", SERVICE_PREFIX, name), string(serviceJson), 0)
		if status.Err() != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, status.Err().Error())
			return
		}

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, "added %s: %s", name, j.HelmChartName)
	}).Methods("POST")

	brokerRouter.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := redisClient.Ping().Result(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	}).Methods("GET")

	broker := &kaboomBroker{
		redisClient: redisClient,
	}
	brokerapi.AttachRoutes(brokerRouter, broker, lager.NewLogger("kaboom"))

	s := &http.Server{
		Addr:    ":80",
		Handler: brokerRouter,
	}
	log.Fatal(s.ListenAndServe())
}
