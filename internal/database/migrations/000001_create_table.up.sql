

CREATE TABLE IF NOT EXISTS user_table(
    id SERIAL PRIMARY KEY,
    email VARCHAR(30) NOT NULL,
    password VARCHAR(300) NOT NULL,
    is_enabled BOOLEAN
);


CREATE TABLE IF NOT EXISTS url_table(
    id SERIAL PRIMARY KEY,
    url VARCHAR(1000) NOT NULL,
    short_url VARCHAR(20) NOT NULL,
    is_alias BOOLEAN NOT NULL,
    clicked INTEGER NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    user_id INTEGER REFERENCES user_table(id)
);