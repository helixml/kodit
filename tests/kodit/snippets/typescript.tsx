import { useState } from 'react';

// Type definitions
type UserRole = 'admin' | 'user' | 'guest';

interface User {
    id: number;
    name: string;
    role: UserRole;
    isActive: boolean;
}

// Utility functions
const formatName = (name: string): string => {
    return name.charAt(0).toUpperCase() + name.slice(1);
};

const calculateAge = (birthYear: number): number => {
    return new Date().getFullYear() - birthYear;
};

// Class definition
class UserManager {
    private users: User[] = [];

    constructor(initialUsers: User[] = []) {
        this.users = initialUsers;
    }

    addUser(user: User): void {
        this.users.push(user);
    }

    getActiveUsers(): User[] {
        return this.users.filter(user => user.isActive);
    }

    findUserById(id: number): User | undefined {
        return this.users.find(user => user.id === id);
    }
}

// Variables
const currentYear: number = new Date().getFullYear();
const defaultRole: UserRole = 'user';
const MAX_USERS: number = 100;

// Example usage
const userManager = new UserManager([
    { id: 1, name: 'john', role: 'admin', isActive: true },
    { id: 2, name: 'jane', role: 'user', isActive: false }
]);

// React component example
const UserList: React.FC = () => {
    const [users, setUsers] = useState<User[]>(userManager.getActiveUsers());

    return (
        <div>
            <h1>Active Users</h1>
            <ul>
                {users.map(user => (
                    <li key={user.id}>
                        {formatName(user.name)} - {user.role}
                    </li>
                ))}
            </ul>
        </div>
    );
};

export { calculateAge, formatName, UserList, UserManager };

