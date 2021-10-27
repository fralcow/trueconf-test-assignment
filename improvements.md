# Bugs
1. createUser(): В поле email записывалось неправильное значение из request
2. enable Recoverer middleware: Включить recoverer middleware. 
3. getUser: возвращать 404, если пользователя нет в БД.

# Refactoring
1. Change ErrInvalidRequest to ErrNotFound to return 404 error
2. Change UpdateUserRequest fields to pointers to allow check for nil fields
