package web

import "database/sql"

func listExpenses(db *sql.DB) ([]Expense, error) {
	rows, err := db.Query(`
		SELECT id, description, amount, created_at
		FROM expenses
		ORDER BY datetime(created_at) DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expenses []Expense
	for rows.Next() {
		var e Expense
		if err := rows.Scan(&e.ID, &e.Description, &e.Amount, &e.CreatedAt); err != nil {
			return nil, err
		}
		expenses = append(expenses, e)
	}
	return expenses, rows.Err()
}

func insertExpense(db *sql.DB, description string, amount float64) (Expense, error) {
	var e Expense
	err := db.QueryRow(`
		INSERT INTO expenses (description, amount)
		VALUES (?, ?)
		RETURNING id, description, amount, created_at
	`, description, amount).Scan(&e.ID, &e.Description, &e.Amount, &e.CreatedAt)
	return e, err
}

func deleteExpense(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM expenses WHERE id = ?`, id)
	return err
}
