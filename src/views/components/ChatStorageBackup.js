export default {
    name: 'ChatStorageBackup',
    data() {
        return {
            loading: false,
            importFile: null,
            importResult: null
        }
    },
    methods: {
        openModal() {
            $('#modalChatStorageBackup').modal('show');
            this.importResult = null;
            this.importFile = null;
        },
        closeModal() {
            $('#modalChatStorageBackup').modal('hide');
        },
        async exportStorage() {
            this.loading = true;
            try {
                const response = await window.http.get('/chat/export', {
                    responseType: 'blob'
                });

                // Create download link
                const blob = new Blob([response.data], { type: 'application/octet-stream' });
                const url = window.URL.createObjectURL(blob);
                const link = document.createElement('a');
                link.href = url;

                // Get filename from Content-Disposition header or use default
                const contentDisposition = response.headers['content-disposition'];
                let filename = 'chatstorage.db';
                if (contentDisposition) {
                    const match = contentDisposition.match(/filename="(.+)"/);
                    if (match) filename = match[1];
                }

                link.download = filename;
                document.body.appendChild(link);
                link.click();
                document.body.removeChild(link);
                window.URL.revokeObjectURL(url);

                showSuccessInfo('Chat storage exported successfully');
            } catch (error) {
                showErrorInfo(error.response?.data?.message || 'Failed to export chat storage');
            } finally {
                this.loading = false;
            }
        },
        handleFileSelect(event) {
            const files = event.target.files;
            if (files && files.length > 0) {
                this.importFile = files[0];
            }
        },
        async importStorage() {
            if (!this.importFile) {
                showErrorInfo('Please select a backup file to import');
                return;
            }

            this.loading = true;
            this.importResult = null;

            try {
                const formData = new FormData();
                formData.append('file', this.importFile);

                const response = await window.http.post('/chat/import', formData, {
                    headers: {
                        'Content-Type': 'multipart/form-data'
                    }
                });

                this.importResult = response.data.results;
                showSuccessInfo('Chat storage imported successfully');
            } catch (error) {
                showErrorInfo(error.response?.data?.message || 'Failed to import chat storage');
            } finally {
                this.loading = false;
            }
        },
        resetImport() {
            this.importFile = null;
            this.importResult = null;
            // Reset file input
            const fileInput = document.getElementById('importFileInput');
            if (fileInput) fileInput.value = '';
        }
    },
    template: `
    <div class="teal card" @click="openModal()" style="cursor: pointer">
        <div class="content">
            <a class="ui teal right ribbon label">Backup</a>
            <div class="header">Chat Storage Backup</div>
            <div class="description">
                Export or import chat history backup
            </div>
        </div>
    </div>

    <!--  Modal ChatStorageBackup  -->
    <div class="ui modal" id="modalChatStorageBackup">
        <i class="close icon"></i>
        <div class="header">
            <i class="database icon"></i>
            Chat Storage Backup
        </div>
        <div class="content">
            <div class="ui two column stackable grid">
                <!-- Export Section -->
                <div class="column">
                    <div class="ui segment">
                        <h3 class="ui header">
                            <i class="download icon"></i>
                            Export Backup
                        </h3>
                        <p>Download your chat history as a SQLite database file. This includes all chats and messages stored locally.</p>
                        <button class="ui primary button"
                                @click="exportStorage"
                                :class="{ loading: loading }">
                            <i class="download icon"></i>
                            Export Chat Storage
                        </button>
                    </div>
                </div>

                <!-- Import Section -->
                <div class="column">
                    <div class="ui segment">
                        <h3 class="ui header">
                            <i class="upload icon"></i>
                            Import Backup
                        </h3>
                        <p>Restore chat history from a backup file. Existing chats will be preserved (merge mode).</p>

                        <div class="ui form">
                            <div class="field">
                                <label>Select Backup File</label>
                                <input type="file"
                                       id="importFileInput"
                                       accept=".db,.sqlite,.sqlite3"
                                       @change="handleFileSelect">
                            </div>
                            <div v-if="importFile" class="field">
                                <div class="ui info message">
                                    <i class="file icon"></i>
                                    Selected: {{ importFile.name }} ({{ (importFile.size / 1024 / 1024).toFixed(2) }} MB)
                                </div>
                            </div>
                            <button class="ui green button"
                                    @click="importStorage"
                                    :class="{ loading: loading, disabled: !importFile }">
                                <i class="upload icon"></i>
                                Import Chat Storage
                            </button>
                            <button v-if="importFile || importResult"
                                    class="ui button"
                                    @click="resetImport">
                                <i class="redo icon"></i>
                                Reset
                            </button>
                        </div>

                        <!-- Import Result -->
                        <div v-if="importResult" class="ui success message" style="margin-top: 1em;">
                            <div class="header">Import Complete</div>
                            <div class="ui list">
                                <div class="item">
                                    <i class="green check icon"></i>
                                    Imported: {{ importResult.imported?.chats || 0 }} chats, {{ importResult.imported?.messages || 0 }} messages
                                </div>
                                <div class="item">
                                    <i class="yellow minus icon"></i>
                                    Skipped (already exist): {{ importResult.skipped?.chats || 0 }} chats, {{ importResult.skipped?.messages || 0 }} messages
                                </div>
                                <div class="item">
                                    <i class="clock icon"></i>
                                    Duration: {{ importResult.duration }}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <div class="ui warning message">
                <div class="header">
                    <i class="exclamation triangle icon"></i>
                    Important Notes
                </div>
                <ul class="list">
                    <li>Export creates a copy of your local chat storage database.</li>
                    <li>Import uses merge mode - existing data is preserved, only new data is added.</li>
                    <li>Media files are not included in the backup - only chat metadata and message text.</li>
                </ul>
            </div>
        </div>
        <div class="actions">
            <div class="ui approve button">Close</div>
        </div>
    </div>
    `
}
