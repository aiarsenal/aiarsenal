package dto

type CalendarSyncDiscoverReq struct {
	ServerUrl string `json:"serverUrl" binding:"required"`
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
}

type CalendarSyncAccountReq struct {
	Name                string `json:"name" binding:"required"`
	ServerUrl           string `json:"serverUrl" binding:"required"`
	CalendarUrl         string `json:"calendarUrl" binding:"required"`
	Username            string `json:"username" binding:"required"`
	Password            string `json:"password"`
	Enabled             uint   `json:"enabled"`
	SyncIntervalMinutes int    `json:"syncIntervalMinutes"`
	ConflictPolicy      string `json:"conflictPolicy"`
}

type CalendarSyncEditReq struct {
	AccountId string `json:"accountId" binding:"required"`
	CalendarSyncAccountReq
}

type CalendarSyncDeleteReq struct {
	AccountId string `json:"accountId" binding:"required"`
}

type CalendarSyncRunReq struct {
	AccountId string `json:"accountId" binding:"required"`
}
