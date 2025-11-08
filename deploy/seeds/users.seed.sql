-- password for both accounts: Password123!
INSERT INTO users (username, email, password_hash, verified, created_at, updated_at)
VALUES (
    'test_1',
    'test1@example.com',
    '$2b$12$B1KAqoq2/p5LmArVEURkj.i1fUBpppliD.BzAeXCihBcI1UHfNLxq',
    TRUE,
    NOW(),
    NOW()
)
ON CONFLICT (username) DO NOTHING;

INSERT INTO users (username, email, password_hash, verified, created_at, updated_at)
VALUES (
    'test_2',
    'test2@example.com',
    '$2b$12$B1KAqoq2/p5LmArVEURkj.i1fUBpppliD.BzAeXCihBcI1UHfNLxq',
    TRUE,
    NOW(),
    NOW()
)
ON CONFLICT (username) DO NOTHING;
