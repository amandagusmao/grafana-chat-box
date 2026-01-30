import React, { useState, useRef, useEffect } from 'react';
import { AppRootProps, GrafanaTheme2, SelectableValue } from '@grafana/data';
import { config, getBackendSrv } from '@grafana/runtime';
import { Button, Select, useStyles2 } from '@grafana/ui';
import { css } from '@emotion/css';

interface Props extends AppRootProps {}

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
}

interface Conversation {
  id: string;
  title: string;
  messages: Message[];
  createdAt: string;
  updatedAt: string;
}

interface DashboardContext {
  [key: string]: any;
}

const STORAGE_KEY = 'grafana-chat-history';
const MAX_CONVERSATIONS = 50;
function getInitialMessage(): string {
  const fullName = (config as any).bootData?.user?.name || (config as any).bootData?.user?.login || '';
  const firstName = fullName.split(' ')[0] || 'usuário';
  return `Olá, ${firstName}! Sou seu assistente para tirar dúvidas e criar dashboards no Grafana. Em que posso te ajudar hoje?`;
}

// --- Utility functions ---

function generateId(): string {
  return Date.now().toString(36) + Math.random().toString(36).substr(2);
}

function generateTitle(msg: string): string {
  const trimmed = msg.trim();
  return trimmed.length <= 40 ? trimmed : trimmed.substring(0, 40) + '...';
}

function deserializeMessages(messages: any[]): Message[] {
  return messages.map((m) => ({ ...m, timestamp: new Date(m.timestamp) }));
}

function loadConversations(): Conversation[] {
  try {
    const data = localStorage.getItem(STORAGE_KEY);
    if (!data) {
      return [];
    }
    const parsed = JSON.parse(data) as Conversation[];
    return parsed.map((conv) => ({
      ...conv,
      messages: deserializeMessages(conv.messages),
    }));
  } catch {
    return [];
  }
}

function saveConversations(conversations: Conversation[]): void {
  try {
    const withContent = conversations.filter((conv) =>
      conv.messages.some((m) => m.role === 'user')
    );
    const sorted = [...withContent]
      .sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
      .slice(0, MAX_CONVERSATIONS);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(sorted));
  } catch {
    // localStorage full or unavailable
  }
}

function createInitialMessage(): Message {
  return {
    id: Date.now().toString() + Math.random(),
    role: 'assistant',
    content: getInitialMessage(),
    timestamp: new Date(),
  };
}

function createConversation(): Conversation {
  const msg = createInitialMessage();
  const now = new Date().toISOString();
  return {
    id: generateId(),
    title: 'Nova conversa',
    messages: [msg],
    createdAt: now,
    updatedAt: now,
  };
}

// --- Styles ---

const getStyles = (theme: GrafanaTheme2) => ({
  container: css`
    display: flex;
    flex-direction: column;
    height: calc(100vh - 120px);
    max-height: calc(100vh - 120px);
    padding: ${theme.spacing(2)};
    overflow: hidden;
  `,
  header: css`
    flex-shrink: 0;
    margin-bottom: ${theme.spacing(2)};
    border-bottom: 1px solid ${theme.colors.border.medium};
    padding-bottom: ${theme.spacing(2)};
  `,
  headerToolbar: css`
    display: flex;
    align-items: center;
    gap: ${theme.spacing(1)};
    margin-top: ${theme.spacing(1)};
  `,
  conversationSelect: css`
    flex: 1;
    min-width: 200px;
  `,
  messagesContainer: css`
    flex: 1;
    overflow-y: auto;
    border: 1px solid ${theme.colors.border.medium};
    border-radius: ${theme.shape.borderRadius()};
    padding: ${theme.spacing(2)};
    margin-bottom: ${theme.spacing(2)};
    background: ${theme.colors.background.primary};
    min-height: 0;
  `,
  message: css`
    margin-bottom: ${theme.spacing(2)};
    padding: ${theme.spacing(1.5)};
    border-radius: ${theme.shape.borderRadius()};
    max-width: 80%;
    word-wrap: break-word;
  `,
  userMessage: css`
    background: ${theme.colors.primary.main};
    color: ${theme.colors.primary.contrastText};
    margin-left: auto;
    text-align: right;
  `,
  assistantMessage: css`
    background: ${theme.colors.background.secondary};
    color: ${theme.colors.text.primary};
    margin-right: auto;
  `,
  inputContainer: css`
    flex-shrink: 0;
    display: flex;
    gap: ${theme.spacing(1)};
    align-items: flex-end;
  `,
  messageInput: css`
    flex: 1;
  `,
  timestamp: css`
    font-size: ${theme.typography.bodySmall.fontSize};
    color: ${theme.colors.text.secondary};
    margin-top: ${theme.spacing(0.5)};
  `,
  emptyState: css`
    text-align: center;
    color: ${theme.colors.text.secondary};
    margin-top: ${theme.spacing(4)};
  `,
});

// --- Component ---

export const ChatAppPage: React.FC<Props> = ({ basename }) => {
  const styles = useStyles2(getStyles);

  const [messages, setMessages] = useState<Message[]>([]);
  const [inputValue, setInputValue] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [conversationHistory, setConversationHistory] = useState<Message[]>([]);
  const [dashboardContext, setDashboardContext] = useState<DashboardContext>({});
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConversationId, setActiveConversationId] = useState<string | null>(null);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const isInitializedRef = useRef(false);

  // Initialize: always start with a new conversation, keeping history in the dropdown
  useEffect(() => {
    const loaded = loadConversations();
    const newConv = createConversation();
    setConversations([newConv, ...loaded]);
    setActiveConversationId(newConv.id);
    setMessages(newConv.messages);
    setConversationHistory(newConv.messages);
    isInitializedRef.current = true;
  }, []);

  // Persist active conversation when messages change (only if user has sent a message)
  useEffect(() => {
    if (!isInitializedRef.current || !activeConversationId) {
      return;
    }
    if (!messages.some((m) => m.role === 'user')) {
      return;
    }

    setConversations((prev) => {
      const firstUserMsg = messages.find((m) => m.role === 'user');
      const updated = prev.map((conv) => {
        if (conv.id !== activeConversationId) {
          return conv;
        }
        return {
          ...conv,
          title: firstUserMsg ? generateTitle(firstUserMsg.content) : conv.title,
          messages: [...messages],
          updatedAt: new Date().toISOString(),
        };
      });
      saveConversations(updated);
      return updated;
    });
  }, [messages, activeConversationId]);

  // Scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // --- Conversation handlers ---

  const handleNewConversation = () => {
    const newConv = createConversation();
    const updated = [newConv, ...conversations];
    setConversations(updated);
    setActiveConversationId(newConv.id);
    setMessages(newConv.messages);
    setConversationHistory(newConv.messages);
    setDashboardContext({});
  };

  const handleSwitchConversation = (selected: SelectableValue<string>) => {
    const convId = selected.value;
    if (!convId || convId === activeConversationId) {
      return;
    }
    const conv = conversations.find((c) => c.id === convId);
    if (!conv) {
      return;
    }
    setActiveConversationId(convId);
    setMessages([...conv.messages]);
    setConversationHistory([...conv.messages]);
    setDashboardContext({});
  };

  const handleDeleteConversation = () => {
    if (!activeConversationId) {
      return;
    }
    if (!window.confirm('Tem certeza que deseja excluir esta conversa?')) {
      return;
    }

    const remaining = conversations.filter((c) => c.id !== activeConversationId);

    if (remaining.length > 0) {
      saveConversations(remaining);
      setConversations(remaining);
      setActiveConversationId(remaining[0].id);
      setMessages([...remaining[0].messages]);
      setConversationHistory([...remaining[0].messages]);
    } else {
      const newConv = createConversation();
      saveConversations([newConv]);
      setConversations([newConv]);
      setActiveConversationId(newConv.id);
      setMessages(newConv.messages);
      setConversationHistory(newConv.messages);
    }
    setDashboardContext({});
  };

  // --- Message handlers ---

  const handleSendMessage = async () => {
    if (!inputValue.trim() || isLoading) {
      return;
    }

    const userMessage: Message = {
      id: Date.now().toString() + Math.random(),
      role: 'user',
      content: inputValue.trim(),
      timestamp: new Date(),
    };
    const newMessages = [...messages, userMessage];
    const newHistory = [...conversationHistory, userMessage];

    // Update UI immediately
    setMessages(newMessages);
    setConversationHistory(newHistory);
    setInputValue('');
    setIsLoading(true);

    try {
      // Use Grafana's backend service to call the plugin's resource handler
      const response = await getBackendSrv()
        .fetch({
          url: '/api/plugins/grafana-chat-assistant/resources/chat',
          method: 'POST',
          data: {
            messages: newHistory,
            dashboardContext: dashboardContext,
          },
          showSuccessAlert: false,
          showErrorAlert: false,
        })
        .toPromise();
      const data = response?.data as any;

      if (data.success) {
        const assistantMessage: Message = {
          id: Date.now().toString() + Math.random(),
          role: 'assistant',
          content: data.message,
          timestamp: new Date(),
        };

        if (data.type === 'dashboard_created') {
          // Dashboard was created successfully
          const dashboardInfo = data.dashboard;
          const successMessage: Message = {
            id: Date.now().toString() + Math.random(),
            role: 'assistant',
            content: `🎉 Dashboard criado com sucesso!\n\n📊 **${dashboardInfo.title}**\n🔗 [Abrir Dashboard](${dashboardInfo.url})\n\nID: ${dashboardInfo.uid}\n\nVocê pode acessar seu dashboard clicando no link acima. Precisa de mais alguma coisa?`,
            timestamp: new Date(),
          };

          setMessages((prev) => [...prev, successMessage]);
          setConversationHistory((prev) => [...prev, successMessage]);

          // Reset dashboard context after successful creation
          setDashboardContext({});
        } else {
          // Regular chat message
          setMessages((prev) => [...prev, assistantMessage]);
          setConversationHistory((prev) => [...prev, assistantMessage]);
        }
      } else {
        throw new Error(data.error || 'Resposta inválida do servidor');
      }
    } catch (error: any) {
      console.error('Chat error:', error);
      const errorMessage: Message = {
        id: Date.now().toString() + Math.random(),
        role: 'assistant',
        content: `❌ Desculpe, ocorreu um erro: ${error.message}\n\nPor favor, tente novamente ou verifique se o servidor está funcionando.`,
        timestamp: new Date(),
      };

      setMessages((prev) => [...prev, errorMessage]);
      setConversationHistory((prev) => [...prev, errorMessage]);
    } finally {
      setIsLoading(false);
      // Restore focus to input after sending message
      setTimeout(() => {
        inputRef.current?.focus();
      }, 100);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSendMessage();
    }
  };

  const escapeHtml = (text: string): string => {
    const map: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' };
    return text.replace(/[&<>"']/g, (ch) => map[ch]);
  };

  const formatMessage = (content: string) => {
    // Escape HTML first to prevent XSS, then apply markdown formatting
    let safe = escapeHtml(content);
    // Bold: **text**
    safe = safe.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');
    // Inline code: `text`
    safe = safe.replace(/`([^`]+)`/g, '<code>$1</code>');
    // Links: [text](url) — only allow http/https URLs
    safe = safe.replace(/\[([^\]]+)\]\((https?:\/\/[^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>');
    // Newlines
    safe = safe.replace(/\n/g, '<br/>');
    return safe;
  };

  // --- Select options ---

  const conversationOptions: Array<SelectableValue<string>> = conversations.map((conv) => ({
    label: conv.title,
    value: conv.id,
    description: new Date(conv.updatedAt).toLocaleString(),
  }));

  const selectedConversation = conversationOptions.find((o) => o.value === activeConversationId);

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h2>Grafana Chat Assistant</h2>
        <div className={styles.headerToolbar}>
          <div className={styles.conversationSelect}>
            <Select
              options={conversationOptions}
              value={selectedConversation}
              onChange={handleSwitchConversation}
              placeholder="Selecionar conversa"
            />
          </div>
          <Button variant="primary" icon="plus" onClick={handleNewConversation}>
            Nova
          </Button>
          <Button variant="destructive" icon="trash-alt" onClick={handleDeleteConversation}>
            Excluir
          </Button>
        </div>
      </div>

      <div className={styles.messagesContainer}>
        {messages.length === 0 ? (
          <div className={styles.emptyState}>
            <p>Inicie uma conversa digitando sua primeira mensagem abaixo</p>
          </div>
        ) : (
          messages.map((message) => (
            <div
              key={message.id}
              className={`${styles.message} ${
                message.role === 'user' ? styles.userMessage : styles.assistantMessage
              }`}
            >
              <div dangerouslySetInnerHTML={{ __html: formatMessage(message.content) }} />
              <div className={styles.timestamp}>{message.timestamp.toLocaleTimeString()}</div>
            </div>
          ))
        )}

        {isLoading && (
          <div className={`${styles.message} ${styles.assistantMessage}`}>
            <div>💭 Pensando...</div>
            <div className={styles.timestamp}>{new Date().toLocaleTimeString()}</div>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      <div className={styles.inputContainer}>
        <textarea
          ref={inputRef}
          className={styles.messageInput}
          style={{
            minHeight: '60px',
            resize: 'vertical',
            padding: '8px',
            fontSize: '14px',
            fontFamily: 'inherit',
            border: '1px solid #ccc',
            borderRadius: '4px',
          }}
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onKeyDown={handleKeyPress}
          placeholder="Digite sua mensagem aqui... (Shift+Enter para nova linha)"
          disabled={isLoading}
        />
        <Button onClick={handleSendMessage} disabled={isLoading || !inputValue.trim()}>
          {isLoading ? '...' : 'Enviar'}
        </Button>
      </div>
    </div>
  );
};
