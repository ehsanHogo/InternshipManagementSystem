package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"internship-management-system/backend/internal/config"
	"internship-management-system/backend/internal/database"
	"internship-management-system/backend/internal/handler"
	appmiddleware "internship-management-system/backend/internal/middleware"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	if err := database.MigrateAndSeed(db); err != nil {
		log.Fatalf("prepare database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("access database connection: %v", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("close database connection: %v", err)
		}
	}()

	healthService := service.NewHealthService(db)
	healthHandler := handler.NewHealthHandler(healthService)
	authHandler := handler.NewAuthHandler(db, cfg.JWT.Secret, time.Duration(cfg.JWT.ExpiresHours)*time.Hour)
	internshipService := service.NewInternshipService(db)
	internshipHandler := handler.NewInternshipHandler(internshipService, cfg.UploadDir)
	managementService := service.NewUniversityManagementService(db)
	managementHandler := handler.NewUniversityManagementHandler(managementService)
	companyAccountService := service.NewCompanyAccountService(db)
	companyAccountHandler := handler.NewCompanyAccountHandler(companyAccountService)
	opportunityService := service.NewOpportunityService(db)
	applicationService := service.NewOpportunityApplicationService(db)
	opportunityHandler := handler.NewOpportunityHandler(opportunityService, applicationService)
	applicationHandler := handler.NewOpportunityApplicationHandler(applicationService, cfg.UploadDir)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), appmiddleware.CORS(cfg.FrontendOrigin))

	api := router.Group("/api")
	api.GET("/health", healthHandler.Get)
	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/company-register", companyAccountHandler.Register)
	api.GET("/auth/me", appmiddleware.RequireAuth(cfg.JWT.Secret), authHandler.Me)
	api.GET("/protected", appmiddleware.RequireAuth(cfg.JWT.Secret), authHandler.Protected)

	authenticated := api.Group("")
	authenticated.Use(appmiddleware.RequireAuth(cfg.JWT.Secret))
	authenticated.GET("/companies", internshipHandler.ListCompanies)

	student := authenticated.Group("/student")
	student.Use(appmiddleware.RequireRole(model.RoleStudent))
	student.GET("/internship-case", internshipHandler.GetCurrentCase)
	student.POST("/internship-case", internshipHandler.CreateOrGetCase)
	student.PUT("/internship-case", internshipHandler.UpdateCase)
	student.POST("/internship-case/preferences", internshipHandler.AddPreference)
	student.PUT("/internship-case/preferences/:id", internshipHandler.UpdatePreference)
	student.DELETE("/internship-case/preferences/:id", internshipHandler.DeletePreference)
	student.POST("/internship-case/submit", internshipHandler.SubmitCase)
	student.GET("/internship-case/weekly-reports", internshipHandler.ListStudentWeeklyReports)
	student.POST("/internship-case/weekly-reports", internshipHandler.CreateWeeklyReport)
	student.PUT("/internship-case/weekly-reports/:id", internshipHandler.UpdateWeeklyReport)
	student.POST("/internship-case/final-report", internshipHandler.UploadFinalReport)
	student.GET("/opportunities", opportunityHandler.ListStudent)
	student.GET("/opportunities/:id", opportunityHandler.GetStudent)
	student.POST("/opportunities/:id/apply", applicationHandler.Apply)
	student.GET("/opportunity-applications", applicationHandler.ListStudent)
	student.GET("/opportunity-applications/:id", applicationHandler.GetStudent)

	university := authenticated.Group("/university")
	university.Use(appmiddleware.RequireRole(model.RoleUniversitySupervisor))
	university.GET("/internship-cases", internshipHandler.ListUniversityCases)
	university.GET("/internship-cases/:id", internshipHandler.GetUniversityCase)
	university.GET("/students", managementHandler.ListStudents)
	university.POST("/students", managementHandler.CreateStudent)
	university.POST("/students/import", managementHandler.ImportStudents)
	university.GET("/professors", managementHandler.ListProfessors)
	university.POST("/professors", managementHandler.CreateProfessor)
	university.POST("/professors/import", managementHandler.ImportProfessors)
	university.GET("/companies", managementHandler.ListCompanies)
	university.POST("/companies", managementHandler.CreateCompany)
	university.GET("/company-supervisors", managementHandler.ListCompanySupervisors)
	university.POST("/company-supervisors", managementHandler.CreateCompanySupervisor)
	university.GET("/professor-assignments", managementHandler.ListProfessorAssignments)
	university.POST("/professor-assignments", managementHandler.CreateProfessorAssignment)
	university.PUT("/professor-assignments/:id", managementHandler.UpdateProfessorAssignment)
	university.POST("/internship-cases/:id/send-to-company", internshipHandler.SendToCompany)
	university.POST("/internship-cases/:id/approve", internshipHandler.ApproveUniversityCase)
	university.POST("/internship-cases/:id/activate", internshipHandler.ActivateUniversityCase)

	company := authenticated.Group("/company")
	company.Use(appmiddleware.RequireRole(model.RoleCompanySupervisor))
	company.GET("/profile", companyAccountHandler.Profile)
	company.GET("/internship-cases", internshipHandler.ListCompanyCases)
	company.GET("/internship-cases/:id", internshipHandler.GetCompanyCase)
	company.POST("/internship-cases/:id/confirm", internshipHandler.ConfirmCompanyCase)
	company.GET("/internship-cases/:id/weekly-reports", internshipHandler.ListCompanyWeeklyReports)
	company.POST("/internship-cases/:id/weekly-reports/:reportId/confirm", internshipHandler.ConfirmWeeklyReport)
	company.GET("/internship-cases/:id/evaluation", internshipHandler.GetCompanyEvaluation)
	company.POST("/internship-cases/:id/evaluation", internshipHandler.CreateCompanyEvaluation)
	company.POST("/opportunities", opportunityHandler.Create)
	company.GET("/opportunities", opportunityHandler.ListCompany)
	company.GET("/opportunities/:id", opportunityHandler.GetCompany)
	company.PUT("/opportunities/:id", opportunityHandler.Update)
	company.POST("/opportunities/:id/close", opportunityHandler.Close)
	company.GET("/opportunities/:id/applications", applicationHandler.ListCompany)
	company.GET("/opportunity-applications/:id", applicationHandler.GetCompany)
	company.POST("/opportunity-applications/:id/accept", applicationHandler.Accept)
	company.POST("/opportunity-applications/:id/reject", applicationHandler.Reject)

	professor := authenticated.Group("/professor")
	professor.Use(appmiddleware.RequireRole(model.RoleProfessor))
	professor.GET("/internship-cases", internshipHandler.ListProfessorCases)
	professor.GET("/internship-cases/:id", internshipHandler.GetProfessorCase)
	professor.POST("/internship-cases/:id/complete", internshipHandler.CompleteProfessorCase)

	authenticated.GET("/files/:id/download", internshipHandler.DownloadFile)

	server := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("backend listening on http://localhost:%s", cfg.AppPort)
		serverErrors <- server.ListenAndServe()
	}()

	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("start HTTP server: %v", err)
		}
	case signal := <-shutdownSignals:
		log.Printf("received %s, shutting down", signal)
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		log.Fatalf("gracefully shut down HTTP server: %v", err)
	}
}
