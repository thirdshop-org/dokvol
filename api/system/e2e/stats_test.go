package e2e

import (
	"testing"
)

// TestE2E_StatsCollection vérifie que le StatsCollector collecte et stocke
// les statistiques volumes et drives dans la base de données.
//
// Prérequis : Docker avec conteneurs en cours d'exécution.
func TestE2E_StatsCollection(t *testing.T) {
	t.Skip("TODO: implement after stats collection refactor (injectable functions)")
}

// TestE2E_StatsAPI vérifie que les endpoints /api/stats/* retournent
// des données cohérentes après une collecte.
func TestE2E_StatsAPI(t *testing.T) {
	t.Skip("TODO: implement with Gin test context + handler package")
}

// TestE2E_StatsCollectorLoop vérifie que le ticker du StatsCollector
// collecte périodiquement et accumule des données.
func TestE2E_StatsCollectorLoop(t *testing.T) {
	t.Skip("TODO: implement after stats collection refactor (injectable functions)")
}
