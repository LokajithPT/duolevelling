package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
)

/* =======================
   DATA MODELS
======================= */

type Payload struct {
	Filename string `json:"filename"`
	Typed    string `json:"typed"`
	Pasted   string `json:"pasted"`
	Real     string `json:"real"`
}

type CheckMeRequest struct {
	ProjectID string `json:"project_id"`
	TaskID    string `json:"task_id"`
}

type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"` // todo | checking | done
}

type Project struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Tasks []Task `json:"tasks"`
}

type ProjectStore struct {
	Projects []Project `json:"projects"`
}

type Streak struct {
	CurrentStreak int    `json:"current_streak"`
	LastCompleted string `json:"last_completed"`
	StreakFreeze  int    `json:"streak_freeze"`
}

/* =======================
   GLOBAL STATE
======================= */

var (
	projectStore ProjectStore
	streak       Streak
	submissions  = make(map[string]Payload)
)

const dataFile = "data.json"

/* =======================
   MAIN
======================= */

func main() {
	// pages
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/ping", pingHandler)
	http.HandleFunc("/dashboard", dashboard)

	// plugin routes
	http.HandleFunc("/submit", submit)
	http.HandleFunc("/checkme", checkme)

	// api
	http.HandleFunc("/api/projectlist", projectListHandler)
	http.HandleFunc("/api/abouttask", aboutTaskHandler)
	http.HandleFunc("/taskstatus", taskStatusHandler)
	http.HandleFunc("/streakstatus", streakStatusHandler)

	// load persistent data
	must(loadProjects())
	must(loadStreak())
	must(loadSubmissions())

	fmt.Println("duoserver running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

/* =======================
   LOAD / SAVE
======================= */

func loadProjects() error {
	f, err := os.Open("projects.json")
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(&projectStore)
}

func saveProjects() error {
	f, err := os.Create("projects.json")
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(projectStore)
}

func loadStreak() error {
	f, err := os.Open("streak.json")
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(&streak)
}

func saveStreak() error {
	f, err := os.Create("streak.json")
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(streak)
}

func saveSubmissions() error {
	f, err := os.Create(dataFile)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(submissions)
}

func loadSubmissions() error {
	f, err := os.Open(dataFile)
	if err != nil {
		return nil // first run
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(&submissions)
}

/* =======================
   BASIC ROUTES
======================= */

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "welcome to duoserver")
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "pong")
	fmt.Println("pong")
}

func dashboard(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/dashboard.html")
	if err != nil {
		http.Error(w, "template error", 500)
		return
	}
	tmpl.Execute(w, submissions)
}

/* =======================
   PLUGIN ROUTES
======================= */

func submit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "expected application/json", 400)
		return
	}

	var p Payload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&p); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	submissions[p.Filename] = p
	must(saveSubmissions())

	fmt.Println("submission:", p.Filename)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "received"})
}

// todo -> checking ONLY
func checkme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req CheckMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}

	if req.ProjectID == "" || req.TaskID == "" {
		http.Error(w, "project_id and task_id required", 400)
		return
	}

	for pi, p := range projectStore.Projects {
		if p.ID == req.ProjectID {
			for ti, t := range p.Tasks {
				if t.ID == req.TaskID {

					if t.Status == "" {
						projectStore.Projects[pi].Tasks[ti].Status = "todo"
					}

					if projectStore.Projects[pi].Tasks[ti].Status == "todo" {
						projectStore.Projects[pi].Tasks[ti].Status = "checking"
						must(saveProjects())
						fmt.Fprintln(w, "submitted for checking")
						return
					}

					fmt.Fprintln(w, "already submitted")
					return
				}
			}
		}
	}

	http.Error(w, "task not found", 404)
}

/* =======================
   API ROUTES
======================= */

func projectListHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(projectStore)
}

func aboutTaskHandler(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project")
	tid := r.URL.Query().Get("task")

	for _, p := range projectStore.Projects {
		if p.ID == pid {
			for _, t := range p.Tasks {
				if t.ID == tid {
					json.NewEncoder(w).Encode(t)
					return
				}
			}
		}
	}

	http.Error(w, "task not found", 404)
}

func taskStatusHandler(w http.ResponseWriter, r *http.Request) {
	out := make(map[string]map[string]string)

	for _, p := range projectStore.Projects {
		out[p.ID] = make(map[string]string)
		for _, t := range p.Tasks {
			out[p.ID][t.ID] = t.Status
		}
	}

	json.NewEncoder(w).Encode(out)
}

func streakStatusHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(streak)
}
