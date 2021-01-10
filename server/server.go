package server

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	"github.com/stuartgrigg/qandagame/engine"
)

const templatesDir = "static/templates/"
const rootTemplateLocation = templatesDir + "home.gohtml"
const gameTemplateLocation = templatesDir + "game.gohtml"
const commonTemplateLocation = templatesDir + "common.gohtml"

type Server struct {
	worker         *engine.Worker
	updatesChannel chan engine.GameStateUpdate
	rootTemplate   *template.Template
	gameTemplate   *template.Template
	wsClients      map[*websocket.Conn]string
}

func NewServer(worker *engine.Worker, updatesChannel chan engine.GameStateUpdate) *Server {
	rootTemplate := template.Must(template.ParseFiles(rootTemplateLocation, commonTemplateLocation))
	gameTemplate := template.Must(template.ParseFiles(gameTemplateLocation, commonTemplateLocation))
	return &Server{
		worker:         worker,
		updatesChannel: updatesChannel,
		rootTemplate:   rootTemplate,
		gameTemplate:   gameTemplate,
		wsClients:      map[*websocket.Conn]string{},
	}
}

type RootData struct {
	Started bool
	Users   []string
}

func (s *Server) loadRootData() RootData {
	return RootData{
		Started: s.worker.GetGameState() != engine.AwaitingStart,
		Users:   s.worker.GetNames(),
	}
}

type GameData struct {
	Lobby               bool
	EnoughUsersToStart  bool
	SubmitQuestion      bool
	SubmitAnswers       bool
	Voting              bool
	VoteResult          bool
	GameResult          bool
	Users               []string
	MyName              string
	MyQuestion          string
	UsersToQuestion     []string
	MyQuestionsToAnswer []*engine.MyQuestionToAnswer
	HasAnswered         bool
	UsersToAnswer       []string
	MyQuestionToVoteOn  *engine.MyQuestionToVoteOn
	HasVoted            bool
	UsersToVote         []string
	VotingResults       *engine.VotingResults
	GameResults         *engine.GameResults
}

func (s *Server) loadGameData(userID string) GameData {
	gameState := s.worker.GetGameState()
	myUser, _ := s.worker.GetMyUser(userID)
	myName := ""
	myQuestion := ""
	users := s.worker.GetNames()
	if myUser != nil {
		myName = myUser.Name
		myQuestion = myUser.QuestionText
	}
	enoughUsersToStart := len(users) >= 3
	var usersToQuestion []string
	if gameState == engine.SubmittingQuestions {
		usersToQuestion = s.worker.GetUsersToQuestion()
	}
	var myQuestionsToAnswer []*engine.MyQuestionToAnswer
	var usersToAnswer []string
	if gameState == engine.SubmittingAnswers {
		myQuestionsToAnswer = s.worker.GetMyQuestionsToAnswer(userID)
		usersToAnswer = s.worker.GetUsersToAnswer()
	}
	var myQuestionToVoteOn *engine.MyQuestionToVoteOn
	var usersToVote []string
	if gameState == engine.Voting {
		myQuestionToVoteOn = s.worker.GetQuestionToVoteOn(userID)
		usersToVote = s.worker.GetUsersToVote()
	}
	var votingResults *engine.VotingResults
	if gameState == engine.VoteResult {
		votingResults = s.worker.GetVotingResults()
	}
	var gameResults *engine.GameResults
	if gameState == engine.GameResult {
		gameResults = s.worker.GetGameResults()
	}
	return GameData{
		Lobby:               gameState == engine.AwaitingStart,
		SubmitQuestion:      gameState == engine.SubmittingQuestions,
		SubmitAnswers:       gameState == engine.SubmittingAnswers,
		Voting:              gameState == engine.Voting,
		VoteResult:          gameState == engine.VoteResult,
		GameResult:          gameState == engine.GameResult,
		Users:               s.worker.GetNames(),
		EnoughUsersToStart:  enoughUsersToStart,
		MyName:              myName,
		MyQuestion:          myQuestion,
		UsersToQuestion:     usersToQuestion,
		MyQuestionsToAnswer: myQuestionsToAnswer,
		HasAnswered:         myUser.Answered,
		UsersToAnswer:       usersToAnswer,
		MyQuestionToVoteOn:  myQuestionToVoteOn,
		HasVoted:            myUser.Voted,
		UsersToVote:         usersToVote,
		VotingResults:       votingResults,
		GameResults:         gameResults,
	}
}

func (s *Server) RootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Request on %s\n", r.URL.Path)
	if r.URL.Path == "/" {
		switch r.Method {
		case "GET":
			s.rootTemplate.Execute(w, s.loadRootData())
		case "POST":
			// Call ParseForm() to parse the raw query and update r.PostForm and r.Form.
			if err := r.ParseForm(); err != nil {
				fmt.Fprintf(w, "ParseForm() err: %v", err)
				return
			}
			remoteIP := r.FormValue("wstrack")
			name := r.FormValue("name")
			action := r.FormValue("action")
			if action == "killgame" {
				s.worker.SubmitRequest(engine.ResetWorkerRequest{}, remoteIP)
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			if s.worker.GetGameState() == engine.AwaitingStart {
				id, _ := uuid.NewV4()
				s.worker.SubmitRequest(engine.NewUserRequest{ID: id.String(), Name: name}, remoteIP)
				http.Redirect(w, r, "game/"+id.String(), http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, "/", http.StatusSeeOther)

		default:
			fmt.Fprintf(w, "Sorry, only GET and POST methods are supported.")
		}
		return
	}

	if strings.HasPrefix(r.URL.Path, "/game/") {
		userID := strings.TrimPrefix(r.URL.Path, "/game/")
		_, ok := s.worker.GetMyUser(userID)
		if ok {
			switch r.Method {
			case "GET":
				s.gameTemplate.Execute(w, s.loadGameData(userID))
			case "POST":
				action := r.FormValue("action")
				remoteIP := r.FormValue("wstrack")
				switch action {
				case "startgame":
					s.worker.SubmitRequest(engine.StartGameRequest{}, remoteIP)
				case "submitquestion":
					questionID, _ := uuid.NewV4()
					s.worker.SubmitRequest(engine.SubmitQuestionRequest{
						ID:     questionID.String(),
						Text:   r.FormValue("question"),
						UserID: userID,
					}, remoteIP)
				case "answerquestions":
					questionIDToAnswer := map[string]engine.MyAnswer{}
					err := r.ParseForm()
					if err != nil {
						fmt.Fprint(w, "Invalid form")
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					for key, value := range r.PostForm {
						if strings.HasPrefix(key, "answer/") && len(value) > 0 {
							answerID, _ := uuid.NewV4()
							questionIDToAnswer[strings.TrimPrefix(key, "answer/")] = engine.MyAnswer{
								AnswerText: value[0],
								ID:         answerID.String(),
							}
						}
					}
					s.worker.SubmitRequest(engine.SubmitAnswersRequest{
						UserID:             userID,
						QuestionIDToAnswer: questionIDToAnswer,
					}, remoteIP)
				case "voteonanswers":
					s.worker.SubmitRequest(engine.SubmitVoteRequest{
						UserID:   userID,
						AnswerID: r.FormValue("answer"),
					}, remoteIP)
				case "nextquestion":
					s.worker.SubmitRequest(engine.NewQuestionRequest{}, remoteIP)
				case "killgame":
					s.worker.SubmitRequest(engine.ResetWorkerRequest{}, remoteIP)
					http.Redirect(w, r, "/", http.StatusSeeOther)
					return
				}

				// Redirect back go a GET to avoid form submission dialogs
				http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
			default:
				fmt.Fprintf(w, "Sorry, only GET and POST methods are supported.")
			}
			return
		}
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
	return
}
