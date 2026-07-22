package main

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email already registered")
	ErrUserNotFound       = errors.New("user not found")
)

type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	Name         string `json:"name"`
	CreatedAt    int64  `json:"createdAt"`
}

type Claims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

type authStore struct {
	db        *sql.DB
	jwtSecret []byte
}

func newAuthStore(ctx context.Context, db *sql.DB, jwtSecret string) (*authStore, error) {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS users (
		id            TEXT PRIMARY KEY,
		email         TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		name          TEXT NOT NULL,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	if err != nil {
		return nil, err
	}
	return &authStore{db: db, jwtSecret: []byte(jwtSecret)}, nil
}

func (a *authStore) Register(ctx context.Context, email, password, name string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	id := newSessionID()
	_, err = a.db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, name) VALUES ($1, $2, $3, $4)`,
		id, email, string(hash), name,
	)
	if err != nil {
		return nil, ErrEmailTaken
	}
	return &User{ID: id, Email: email, Name: name, CreatedAt: time.Now().UnixMilli()}, nil
}

func (a *authStore) Login(ctx context.Context, email, password string) (*User, error) {
	var u User
	var hash string
	err := a.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, name, EXTRACT(EPOCH FROM created_at)*1000 FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &hash, &u.Name, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return &u, nil
}

func (a *authStore) GenerateToken(user *User) (string, error) {
	claims := &Claims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtSecret)
}

func (a *authStore) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		return a.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func (a *authStore) GetUserByID(ctx context.Context, id string) (*User, error) {
	var u User
	err := a.db.QueryRowContext(ctx,
		`SELECT id, email, name, EXTRACT(EPOCH FROM created_at)*1000 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

var seedUsers = []struct {
	email    string
	password string
	name     string
}{
	{"admin@wacalls.com", "admin123", "Administrador"},
	{"operador@wacalls.com", "operador123", "Operador"},
	{"demo@wacalls.com", "demo123", "Demo"},
}

func (a *authStore) Seed(ctx context.Context) error {
	var count int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	for _, u := range seedUsers {
		if _, err := a.Register(ctx, u.email, u.password, u.name); err != nil {
			return err
		}
	}
	return nil
}
