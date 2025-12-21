import React, { useState, useRef, useEffect } from 'react';
import { AppRootProps, GrafanaTheme2 } from '@grafana/data';
import { getBackendSrv } from '@grafana/runtime';
import { Button, useStyles2 } from '@grafana/ui';
import { css } from '@emotion/css';

interface Props extends AppRootProps {}

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
}

interface DashboardContext {
  [key: string]: any;
}

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
  `
});

export const ChatAppPage: React.FC<Props> = ({ basename }) => {
  const styles = useStyles2(getStyles);
  
  const initialMessage: Message = {
    id: '1',
    role: 'assistant',
    content: 'Olá! Sou seu assistente para criar dashboards no Grafana. Vou ajudar você a configurar tudo de forma simples e intuitiva. Em que posso te ajudar hoje?',
    timestamp: new Date()
  };
  
  const [messages, setMessages] = useState<Message[]>([initialMessage]);
  const [inputValue, setInputValue] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [conversationHistory, setConversationHistory] = useState<Message[]>([initialMessage]);
  const [dashboardContext, setDashboardContext] = useState<DashboardContext>({});
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);


  const addMessage = (content: string, role: 'user' | 'assistant') => {
    const newMessage: Message = {
      id: Date.now().toString() + Math.random(),
      content,
      role,
      timestamp: new Date()
    };
    setMessages(prev => [...prev, newMessage]);
  };

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const handleSendMessage = async () => {
    if (!inputValue.trim() || isLoading) return;

    const userMessage: Message = {
      id: Date.now().toString() + Math.random(),
      role: 'user',
      content: inputValue.trim(),
      timestamp: new Date()
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
      const response = await getBackendSrv().fetch({
        url: '/api/plugins/grafana-chat-assistant/resources/chat',
        method: 'POST',
        data: {
          messages: newHistory,
          dashboardContext: dashboardContext
        },
        showSuccessAlert: false,
        showErrorAlert: false
      }).toPromise();
      const data = response?.data as any;

      if (data.success) {
        const assistantMessage: Message = {
          id: Date.now().toString() + Math.random(),
          role: 'assistant',
          content: data.message,
          timestamp: new Date()
        };
        
        if (data.type === 'dashboard_created') {
          // Dashboard was created successfully
          const dashboardInfo = data.dashboard;
          const successMessage: Message = {
            id: Date.now().toString() + Math.random(),
            role: 'assistant',
            content: `🎉 Dashboard criado com sucesso!\n\n📊 **${dashboardInfo.title}**\n🔗 [Abrir Dashboard](${dashboardInfo.url})\n\nID: ${dashboardInfo.uid}\n\nVocê pode acessar seu dashboard clicando no link acima. Precisa de mais alguma coisa?`,
            timestamp: new Date()
          };
          
          setMessages(prev => [...prev, successMessage]);
          setConversationHistory(prev => [...prev, successMessage]);
          
          // Reset dashboard context after successful creation
          setDashboardContext({});
        } else {
          // Regular chat message
          setMessages(prev => [...prev, assistantMessage]);
          setConversationHistory(prev => [...prev, assistantMessage]);
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
        timestamp: new Date()
      };
      
      setMessages(prev => [...prev, errorMessage]);
      setConversationHistory(prev => [...prev, errorMessage]);
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

  const formatMessage = (content: string) => {
    // Simple markdown-like formatting
    return content
      .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
      .replace(/\n/g, '<br/>');
  };

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h2>Grafana Chat Assistant</h2>
        <p>Converse com seu assistente para criar e configurar dashboards</p>
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
              <div className={styles.timestamp}>
                {message.timestamp.toLocaleTimeString()}
              </div>
            </div>
          ))
        )}
        
        {isLoading && (
          <div className={`${styles.message} ${styles.assistantMessage}`}>
            <div>💭 Pensando...</div>
            <div className={styles.timestamp}>
              {new Date().toLocaleTimeString()}
            </div>
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
            borderRadius: '4px'
          }}
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onKeyDown={handleKeyPress}
          placeholder="Digite sua mensagem aqui... (Shift+Enter para nova linha)"
          disabled={isLoading}
        />
        <Button 
          onClick={handleSendMessage} 
          disabled={isLoading || !inputValue.trim()}
        >
          {isLoading ? '...' : 'Enviar'}
        </Button>
      </div>
    </div>
  );
};