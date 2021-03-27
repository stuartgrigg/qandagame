# qandagame
A multiplayer web-based question and answer game that will probably make you laugh.

Players each ask the group a question. They then all answer each other's questions. Finally, the players
vote on which answer is best. Whoever gets the most votes in total wins.

Whether you think humour or wisdom is the best approach is up to you.

Its recommended to have a video chat open with your friends whilst playing if you are not co-located.

## Setup
The app is a Go program that exposes a simple HTTP server. This can be run on your local machine on port 12121 with

```sh
env SERVER_ADDRESS=localhost:12121 go run main.go
```

To then expose things globally first install [localtunnel](https://github.com/localtunnel/localtunnel) with

```sh
npm install -g localtunnel
```
and then start forwarding to the app with
```sh
lt --port 12121
```

Using the free tier of ngrok is not recommended as the game will breach the imposed connection limit. You could also host
the game on low spec VM on your hosting provider of choice.

## Contributing
Contributions welcome! Currently the code is in a prototype state so the first task will be making it more maintainable. 

## Licence
The game is GPLv3 licenced to ensure that all improvements to the game are shared for the benefit of all.


