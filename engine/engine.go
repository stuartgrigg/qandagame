package engine

import (
	"sort"

	"github.com/stuartgrigg/qandagame/logging"
)

type Request interface{}

type Response interface{}

type RequestAndResponseChannel struct {
	request         Request
	responseChannel chan Response
	clientID        string
}

type question struct {
	id           string
	text         string
	userToAnswer map[string]*answer
}

type answer struct {
	id         string
	answerText string
	voterNames []string
}

type user struct {
	id       string
	name     string
	question *question
	answered bool
	voted    bool
	score    int
}

type NewUserRequest struct {
	ID   string
	Name string
}

type StartGameRequest struct{}

type SubmitQuestionRequest struct {
	ID     string
	Text   string
	UserID string
}

type SubmitAnswersRequest struct {
	UserID             string
	QuestionIDToAnswer map[string]MyAnswer
}

type MyAnswer struct {
	ID         string
	AnswerText string
}

type SubmitVoteRequest struct {
	UserID   string
	AnswerID string
}

type NewQuestionRequest struct{}

type ResetWorkerRequest struct{}

type Worker struct {
	logger           *logging.Logger
	requests         chan RequestAndResponseChannel
	gameStateUpdates chan GameStateUpdate
	users            map[string]*user
	state            GameState
	usersToBeVoted   []string
	votingUserIndex  int
}

func NewWorker(logger *logging.Logger) (*Worker, chan GameStateUpdate) {
	gameStateUpdates := make(chan GameStateUpdate, 1)
	return &Worker{
		logger:           logger,
		requests:         make(chan RequestAndResponseChannel),
		gameStateUpdates: gameStateUpdates,
		users:            map[string]*user{},
		state:            AwaitingStart,
	}, gameStateUpdates
}

func (w *Worker) resetWorker() {
	w.users = map[string]*user{}
	w.usersToBeVoted = []string{}
	w.votingUserIndex = 0
}

type GameStateUpdate struct {
	GameState GameState
	ClientID  string
}

type GameState int

const (
	AwaitingStart GameState = iota + 1
	SubmittingQuestions
	SubmittingAnswers
	Voting
	VoteResult
	GameResult
)

func (w *Worker) Run() {
	for req := range w.requests {
		w.handleRequest(req)
	}
}

func (w *Worker) SubmitRequest(r Request, clientID string) Response {
	randr := RequestAndResponseChannel{
		request:         r,
		responseChannel: make(chan Response),
		clientID:        clientID,
	}
	w.requests <- randr
	response := <-randr.responseChannel
	return response
}

func (w *Worker) GetGameState() GameState {
	return w.state
}

func (w *Worker) GetNames() []string {
	var names []string
	for _, user := range w.users {
		names = append(names, user.name)
	}
	return names
}

type MyUser struct {
	Name         string
	ID           string
	QuestionText string
	Answered     bool
	Voted        bool
}

type MyQuestionToAnswer struct {
	QuestionText string
	ID           string
}

type MyQuestionToVoteOn struct {
	QuestionText    string
	ID              string
	AnswersToVoteOn []*MyAnswerToVoteOn
}

type MyAnswerToVoteOn struct {
	AnswerText string
	ID         string
}

type VotingResult struct {
	AnswerText string
	Voters     []string
	Writer     string
}

type VotingResults struct {
	QuestionText   string
	QuestionerName string
	Results        []VotingResult
}

type PlayerScore struct {
	Name  string
	Score int
}

type GameResults struct {
	PlayerScores []PlayerScore
}

func (w *Worker) GetMyUser(userID string) (*MyUser, bool) {
	user, ok := w.users[userID]
	if !ok {
		return nil, false
	}
	questionText := ""
	if user.question != nil {
		questionText = user.question.text
	}

	return &MyUser{
		Name:         user.name,
		ID:           user.id,
		QuestionText: questionText,
		Answered:     user.answered,
		Voted:        user.voted,
	}, true
}

func (w *Worker) GetMyQuestionsToAnswer(userID string) []*MyQuestionToAnswer {
	out := []*MyQuestionToAnswer{}
	for _, user := range w.users {
		if user.id != userID && user.question != nil {
			out = append(out, &MyQuestionToAnswer{
				ID:           user.question.id,
				QuestionText: user.question.text,
			})
		}
	}
	return out
}

func (w *Worker) GetUsersToQuestion() []string {
	out := []string{}
	for _, user := range w.users {
		if user.question == nil {
			out = append(out, user.name)
		}
	}
	return out
}

func (w *Worker) GetUsersToAnswer() []string {
	out := []string{}
	for _, user := range w.users {
		if !user.answered {
			out = append(out, user.name)
		}
	}
	return out
}

func (w *Worker) GetUsersToVote() []string {
	out := []string{}
	for _, user := range w.users {
		if !user.voted {
			out = append(out, user.name)
		}
	}
	return out
}

func (w *Worker) getUserToVoteOn() *user {
	if len(w.usersToBeVoted) == 0 {
		return nil
	}
	userToVoteOn, ok := w.users[w.usersToBeVoted[w.votingUserIndex]]
	if !ok || userToVoteOn.question == nil {
		return nil
	}
	return userToVoteOn
}

func (w *Worker) GetQuestionToVoteOn(userID string) *MyQuestionToVoteOn {
	userToVoteOn := w.getUserToVoteOn()
	if userToVoteOn == nil {
		return nil
	}

	answersToVoteOn := []*MyAnswerToVoteOn{}
	for answerUserID, answer := range userToVoteOn.question.userToAnswer {
		if answerUserID == userID {
			continue
		}
		answersToVoteOn = append(answersToVoteOn, &MyAnswerToVoteOn{
			AnswerText: answer.answerText,
			ID:         answer.id,
		})
	}

	return &MyQuestionToVoteOn{
		QuestionText:    userToVoteOn.question.text,
		ID:              userToVoteOn.question.id,
		AnswersToVoteOn: answersToVoteOn,
	}
}

func (w *Worker) GetVotingResults() *VotingResults {
	userToVoteOn := w.getUserToVoteOn()
	if userToVoteOn == nil {
		return nil
	}
	votingResults := &VotingResults{
		QuestionText:   userToVoteOn.question.text,
		QuestionerName: userToVoteOn.name,
	}
	for answerUserID, answer := range userToVoteOn.question.userToAnswer {
		var writerName string
		if writer, ok := w.users[answerUserID]; ok {
			writerName = writer.name
		}
		votingResult := VotingResult{
			AnswerText: answer.answerText,
			Voters:     answer.voterNames,
			Writer:     writerName,
		}
		votingResults.Results = append(votingResults.Results, votingResult)
	}
	sort.Slice(votingResults.Results, func(i int, j int) bool {
		return len(votingResults.Results[i].Voters) > len(votingResults.Results[j].Voters)
	})
	return votingResults
}

func (w *Worker) GetGameResults() *GameResults {
	userSlice := []*user{}
	for _, user := range w.users {
		userSlice = append(userSlice, user)
	}
	sort.Slice(userSlice, func(i int, j int) bool {
		return userSlice[i].score > userSlice[j].score
	})
	playerScores := make([]PlayerScore, len(userSlice))
	for i, user := range userSlice {
		playerScores[i] = PlayerScore{
			Name:  user.name,
			Score: user.score,
		}
	}
	return &GameResults{PlayerScores: playerScores}
}

func (w *Worker) handleRequest(randr RequestAndResponseChannel) {
	r := randr.request
	switch req := r.(type) {
	case NewUserRequest:
		{
			if w.state == AwaitingStart {
				w.users[req.ID] = &user{
					id:   req.ID,
					name: req.Name,
				}
				w.updateGameState(AwaitingStart, randr.clientID)
			}
		}
	case SubmitQuestionRequest:
		{
			if w.state == SubmittingQuestions {
				user, ok := w.users[req.UserID]
				if ok {
					user.question = &question{
						id:           req.ID,
						text:         req.Text,
						userToAnswer: map[string]*answer{},
					}
				}
				userWithNoQuestion := false
				for _, user := range w.users {
					if user.question == nil {
						userWithNoQuestion = true
						break
					}
				}
				if !userWithNoQuestion {
					w.updateGameState(SubmittingAnswers, randr.clientID)
				} else {
					// Push the "hasQuestioned" list
					w.updateGameState(SubmittingQuestions, randr.clientID)
				}
			}
		}
	case SubmitAnswersRequest:
		{
			if w.state == SubmittingAnswers {
				myUser, ok := w.users[req.UserID]
				if ok {
					myUser.answered = true
					userNotAnswered := false
					for _, user := range w.users {
						if !user.answered {
							userNotAnswered = true
						}
						if myAnswer, ok := req.QuestionIDToAnswer[user.question.id]; ok {
							user.question.userToAnswer[myUser.id] = &answer{
								id:         myAnswer.ID,
								answerText: myAnswer.AnswerText,
							}
						}
					}
					if !userNotAnswered {
						w.updateGameState(Voting, randr.clientID)
						w.usersToBeVoted = []string{}
						for _, user := range w.users {
							w.usersToBeVoted = append(w.usersToBeVoted, user.id)
							w.votingUserIndex = 0
						}
					} else {
						// Push the "hasAnswered" list
						w.updateGameState(SubmittingAnswers, randr.clientID)
					}
				}
			}
		}
	case SubmitVoteRequest:
		{
			if w.state == Voting {
				user, ok := w.users[req.UserID]
				if ok {
					userToVoteOn, ok := w.users[w.usersToBeVoted[w.votingUserIndex]]
					if ok || userToVoteOn.question != nil {
						for answerUserID, answer := range userToVoteOn.question.userToAnswer {
							if answer.id == req.AnswerID {
								answer.voterNames = append(answer.voterNames, user.name)
								if answerUser, ok := w.users[answerUserID]; ok {
									answerUser.score++
								}
								user.voted = true
							}
						}
					}
					// If all the users have voted move to the vote result screen
					userNotVoted := false
					for _, user := range w.users {
						if !user.voted {
							userNotVoted = true
						}
					}
					if !userNotVoted {
						for _, user := range w.users {
							user.voted = false
						}
						w.updateGameState(VoteResult, randr.clientID)
					} else {
						// Push the "hasVoted" list
						w.updateGameState(Voting, randr.clientID)
					}
				}
			}
		}
	case NewQuestionRequest:
		{
			if w.state == VoteResult {
				if w.votingUserIndex < len(w.usersToBeVoted)-1 {
					w.votingUserIndex++
					w.updateGameState(Voting, randr.clientID)
				} else {
					w.updateGameState(GameResult, randr.clientID)
				}
			}
		}
	case StartGameRequest:
		if (len(w.users)) >= 3 {
			w.updateGameState(SubmittingQuestions, randr.clientID)
			w.logger.LogGameStarted()
		}
	case ResetWorkerRequest:
		w.resetWorker()
		w.updateGameState(AwaitingStart, randr.clientID)
	}
	randr.responseChannel <- struct{}{}
}

func (w *Worker) updateGameState(state GameState, clientID string) {
	w.state = state
	go func() {
		w.gameStateUpdates <- GameStateUpdate{
			GameState: state,
			ClientID:  clientID,
		}
	}()
}
