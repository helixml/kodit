import { useState } from 'react';
import { Button, Select, Card, TextInput } from './components';

const themeOptions = [
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
];

export function SettingsPage() {
  const [name, setName] = useState('');
  const [theme, setTheme] = useState('light');

  const handleSave = () => {
    console.log('Saving settings:', { name, theme });
  };

  return (
    <div className="settings-page">
      <Card title="User Settings">
        <TextInput value={name} onChange={setName} placeholder="Enter name" />
        <Select
          options={themeOptions}
          value={theme}
          onChange={setTheme}
          placeholder="Select theme"
        />
        <Button label="Save" onClick={handleSave} />
      </Card>
    </div>
  );
}
