import { useState, useEffect } from 'react';

interface Todo {
  id: number;
  text: string;
  completed: boolean;
}

function App() {
  const [todos, setTodos] = useState<Todo[]>([]);
  const [input, setInput] = useState('');

  useEffect(() => {
    const saved = localStorage.getItem('todos');
    if (saved) setTodos(JSON.parse(saved));
  }, []);

  const addTodo = () => {
    if (!input.trim()) return;
    const newTodos = [...todos, { id: Date.now(), text: input, completed: false }];
    setTodos(newTodos);
    localStorage.setItem('todos', JSON.stringify(newTodos));
    setInput('');
  };

  const toggleTodo = (id: number) => {
    const newTodos = todos.map(t => t.id === id ? {...t, completed: !t.completed} : t);
    setTodos(newTodos);
    localStorage.setItem('todos', JSON.stringify(newTodos));
  };

  const deleteTodo = (id: number) => {
    const newTodos = todos.filter(t => t.id !== id);
    setTodos(newTodos);
    // BUG: Missing localStorage save - todos come back after refresh!
  };

  return (
    <div style={{maxWidth: '600px', margin: '0 auto', padding: '20px'}}>
      <h1>Todo App</h1>
      <div style={{display: 'flex', gap: '10px', marginBottom: '20px'}}>
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyPress={(e) => e.key === 'Enter' && addTodo()}
          placeholder="Add a todo..."
          style={{flex: 1, padding: '8px'}}
        />
        <button onClick={addTodo}>Add</button>
      </div>
      <ul style={{listStyle: 'none', padding: 0}}>
        {todos.map(todo => (
          <li key={todo.id} style={{padding: '10px', borderBottom: '1px solid #ccc', display: 'flex', alignItems: 'center', gap: '10px'}}>
            <input
              type="checkbox"
              checked={todo.completed}
              onChange={() => toggleTodo(todo.id)}
            />
            <span style={{flex: 1, textDecoration: todo.completed ? 'line-through' : 'none'}}>
              {todo.text}
            </span>
            <button onClick={() => deleteTodo(todo.id)}>Delete</button>
          </li>
        ))}
      </ul>
    </div>
  );
}

export default App;