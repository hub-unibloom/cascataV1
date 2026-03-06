// Package extensions — Job Diário de Segurança (0.8.6)
// Ref: SRS Req-2.20.5
// Tolerante à falha (Zero Error Bubble): Não derruba a infraestrutura do Control Plane em caso de offline das APIs da OSV ou Github.
package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// AdvisoryDTO mapeia a formatação universal do Vulnerability Report
type AdvisoryDTO struct {
	IDs      []string
	Severity string // "CRITICAL", "HIGH", "MEDIUM", "LOW", "INFORMATIONAL"
	Summary  string
}

// CVECheckEvent estende o ExtensionEvent para comportar a checagem falha/ignorada.
type CVECheckEvent struct {
	Extension string
	Result    string // 'skipped'
	Reason    string
}

// OSVQueryResult representa uma fração da response do schema JSON da base OSV.dev
type OSVQueryResult struct {
	Vulns []struct {
		ID      string `json:"id"`
		Summary string `json:"summary"`
		Details string `json:"details"`
		// Na API Real OSV, o Severity normalmente vem num array 'database_specific.severity'
		// Para propósitos da demonstração do Motor de Extensões, abstrairemos a extração exata.
	} `json:"vulns"`
}

// ExtensionAdvisoryMap amarra o DB Name com o package tracker nas bases de CVE Open Source.
var extensionAdvisoryMap = map[string]string{
	"postgis":     "postgis",
	"pg_cron":     "pg_cron",
	// plpgsql é core postgresql (coberto nativamente fora do Extension Tracker)
}

// CVEMonitor escaneia vulnerabilidades em background.
type CVEMonitor struct {
	githubToken string // O Token DEVE vir do OpenBao (Secret Manager) na inicialização
	httpClient  *http.Client
	repo        *ExtensionRepository
	events      EventPublisher
	log         Logger
}

// NewCVEMonitor injeção limpa de dependências (Zero secrets hardcoded)
func NewCVEMonitor(githubToken string, repo *ExtensionRepository, ev EventPublisher, log Logger) *CVEMonitor {
	return &CVEMonitor{
		githubToken: githubToken,
		httpClient:  &http.Client{},
		repo:        repo,
		events:      ev,
		log:         log,
	}
}

// Run é a Entrypoint do Job agendado para rodar iterativamente a cada 24H.
func (m *CVEMonitor) Run(ctx context.Context) {
	// (Simulação de Load) activeExtensions buscaria de forma DISTINCT todas que tem status 'active' hoje
	// activeExtensions, err := m.repo.GetAllActiveExtensions(ctx) ...
	// Para focar na infraestrutura exigida na Task 0.8.6:
	
	uniqueActiveExtensions := []string{"postgis", "pg_cron"}

	for _, extName := range uniqueActiveExtensions {
		advisories, source, err := m.fetchAdvisories(ctx, extName)
		
		if err != nil {
			// Não bloqueia/derruba a pipeline - Redpanda Audit 'skipped'
			m.log.Warn("cve_advisory_fetch_skipped", "extension", extName, "error", err)

			_ = m.events.Publish(ctx, ExtensionEvent{
				Extension:     extName,
				Action:        "cve_check_skipped",
				Result:        "skipped",
				FailureReason: err.Error(),
			})
			continue
		}

		for _, adv := range advisories {
			// MOCK Affected Tenants array (ex: todo mundo que tem o postgis habilitado)
			mockAffectedTenants := []string{"uuid-tenant-1", "uuid-tenant-2"}
			m.processAdvisory(ctx, adv, extName, source, mockAffectedTenants)
		}
	}
}

// fetchAdvisories tenta OSV primeiro, caso falhe, fallback p/ Github Advisories
func (m *CVEMonitor) fetchAdvisories(ctx context.Context, extName string) ([]AdvisoryDTO, string, error) {
	packageName, hasMapping := extensionAdvisoryMap[extName]
	if !hasMapping {
		return nil, "", nil // N/A Tracking map
	}

	// 1. OSV Query (Primária)
	osvResults, err := m.queryOSV(ctx, packageName)
	if err == nil {
		return osvResults, "osv.dev", nil
	}
	m.log.Warn("osv_fetch_failed_trying_github", "extension", extName, "error", err)

	// 2. Github Query (Secundária)
	ghResults, err := m.queryGitHubAdvisory(ctx, packageName)
	if err == nil {
		return ghResults, "github_advisory", nil
	}

	return nil, "", fmt.Errorf("todas as fontes de CVE ficaram indisponíveis para a ext '%s': %w", extName, err)
}

// queryOSV faz fetch puro JSON POST
func (m *CVEMonitor) queryOSV(ctx context.Context, packageName string) ([]AdvisoryDTO, error) {
	reqBody := fmt.Sprintf(`{"package": {"name": "%s", "ecosystem": "OSS-Fuzz"}}`, packageName)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.osv.dev/v1/query", strings.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil, fmt.Errorf("osv request failed")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var osv OSVQueryResult
	_ = json.Unmarshal(body, &osv)

	var dto []AdvisoryDTO
	for _, v := range osv.Vulns {
		// Mock parse severity na OSV real aqui
		dto = append(dto, AdvisoryDTO{
			IDs:      []string{v.ID},
			Severity: "HIGH", // Simplified string extractor
			Summary:  v.Summary,
		})
	}

	return dto, nil
}

func (m *CVEMonitor) queryGitHubAdvisory(ctx context.Context, packageName string) ([]AdvisoryDTO, error) {
	if m.githubToken == "" {
		return nil, fmt.Errorf("github token from OpenBao is empty/not configured")
	}
	// Simulated GET request to github GQL/Rest
	return []AdvisoryDTO{}, nil
}

// processAdvisory toma as ações correspondentes a escala de perigo exigida
func (m *CVEMonitor) processAdvisory(ctx context.Context, adv AdvisoryDTO, extName, source string, affectedTenants []string) {
	for _, tenantID := range affectedTenants {
		switch adv.Severity {
		case "CRITICAL", "HIGH":
			m.createUrgentAlert(ctx, tenantID, adv, extName)
		case "MEDIUM":
			m.createWarningAlert(ctx, tenantID, adv, extName)
		case "LOW", "INFORMATIONAL":
			// Sem alert badge, cai direto pro log imutável nas linhas a seguir
		}

		b, _ := json.Marshal(adv.IDs)
		_ = m.events.Publish(ctx, ExtensionEvent{
			TenantID:   tenantID,
			Extension:  extName,
			Action:     "cve_alert",
			Result:     "success",
			ExecutedBy: "system_cve_monitor",
			// Como definimos CveIDs Nullable na Table, publicaremos raw JSON no ClickHouse Array format
			FailureReason: fmt.Sprintf("severity:%s|source:%s|cve_ids:%s", adv.Severity, source, string(b)),
		})
	}
}

// Stubs de Mensageria Painel para UX:
func (m *CVEMonitor) createUrgentAlert(ctx context.Context, tenant, adv AdvisoryDTO, extName string) {
	// Notifica badge vermelho + painel central ("Atualizar docker ...")
	_ = tenant
}

func (m *CVEMonitor) createWarningAlert(ctx context.Context, tenant, adv AdvisoryDTO, extName string) {
	// Notifica badge amarelo ("Monitorar correção...")
	_ = tenant
}
