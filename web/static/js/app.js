document.addEventListener('DOMContentLoaded', function() {
    const form = document.getElementById('upload-form');
    const redInput = document.getElementById('red-input');
    const nirInput = document.getElementById('nir-input');
    const redPreview = document.getElementById('red-preview');
    const nirPreview = document.getElementById('nir-preview');
    const loading = document.getElementById('loading');
    const analyzeBtn = document.getElementById('analyze-btn');
    const resultPlaceholder = document.getElementById('result-placeholder');
    const resultContent = document.getElementById('result-content');

    redInput.addEventListener('change', function(e) {
        handlePreview(e.target.files[0], redPreview);
    });

    nirInput.addEventListener('change', function(e) {
        handlePreview(e.target.files[0], nirPreview);
    });

    function handlePreview(file, container) {
        if (!file) return;
        const reader = new FileReader();
        reader.onload = function(e) {
            container.innerHTML = `<img src="${e.target.result}" alt="preview">`;
        };
        reader.readAsDataURL(file);
    }

    form.addEventListener('submit', async function(e) {
        e.preventDefault();

        if (!redInput.files[0] || !nirInput.files[0]) {
            alert('请选择红光和红外光图片');
            return;
        }

        loading.classList.remove('hidden');
        analyzeBtn.disabled = true;
        resultPlaceholder.classList.add('hidden');
        resultContent.classList.add('hidden');

        const formData = new FormData();
        formData.append('red', redInput.files[0]);
        formData.append('nir', nirInput.files[0]);
        formData.append('grid_size', document.getElementById('grid-size').value);

        try {
            const response = await fetch('/api/upload', {
                method: 'POST',
                body: formData
            });

            const result = await response.json();

            if (!response.ok) {
                throw new Error(result.error || '分析失败');
            }

            displayResult(result);
        } catch (error) {
            alert('分析失败: ' + error.message);
            resultPlaceholder.classList.remove('hidden');
        } finally {
            loading.classList.add('hidden');
            analyzeBtn.disabled = false;
        }
    });

    function displayResult(result) {
        resultPlaceholder.classList.add('hidden');
        resultContent.classList.remove('hidden');

        document.getElementById('stat-size').textContent =
            `${result.ndvi.width} × ${result.ndvi.height}`;

        document.getElementById('stat-mean').textContent =
            result.ndvi.mean.toFixed(4);

        document.getElementById('stat-range').textContent =
            `[${result.ndvi.min.toFixed(3)}, ${result.ndvi.max.toFixed(3)}]`;

        document.getElementById('stat-bad').textContent =
            `${result.grid.bad_count} / ${result.grid.total_count}`;

        document.getElementById('ndvi-image').src = result.ndvi_image;

        document.getElementById('grid-size-info').textContent = result.grid.grid_size;
        document.getElementById('grid-rows').textContent = result.grid.rows;
        document.getElementById('grid-cols').textContent = result.grid.cols;
        document.getElementById('grid-total').textContent = result.grid.total_count;

        const badCellsList = document.getElementById('bad-cells-list');
        if (result.grid.bad_cells.length === 0) {
            badCellsList.innerHTML = '<div style="padding: 20px; text-align: center; color: #68d391;">🎉 所有区域长势良好！</div>';
        } else {
            const topBad = result.grid.bad_cells.slice(0, 50);
            badCellsList.innerHTML = topBad.map(cell => `
                <div class="bad-cell-item">
                    <span class="cell-position">第${cell.row}行 · 第${cell.col}列</span>
                    <span class="cell-ndvi">NDVI: ${cell.mean.toFixed(4)}</span>
                    <span class="cell-status status-${cell.status}">${getStatusText(cell.status)}</span>
                </div>
            `).join('');
        }
    }

    function getStatusText(status) {
        const map = {
            'excellent': '优秀',
            'good': '良好',
            'moderate': '中等',
            'poor': '较差',
            'bad': '差'
        };
        return map[status] || status;
    }
});
