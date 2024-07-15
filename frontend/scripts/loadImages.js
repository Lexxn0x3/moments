let currentPhotoIndex = 0;
let photos = [];

const fileInput = document.getElementById('file-upload');
const status = document.getElementById('status');
const uploadButton = document.getElementById('upload-button');
const fileUploadLabel = document.getElementById('file-upload-label');

status.innerHTML = "Let's upload some files!";

fileInput.addEventListener('change', () => {
	if (fileInput.files.length > 0) {
		uploadButton.style.display = "inline-block";
		fileUploadLabel.textContent = `Selected ${fileInput.files.length} file(s)`;
	} else {
		uploadButton.style.display = "none";
		fileUploadLabel.textContent = 'Select files';
	}
});

const form = document.getElementById('upload-form');
form.addEventListener('submit', async (e) => {
    e.preventDefault();

    const files = fileInput.files;
	const uploadProgress = document.getElementById('upload-progress');

    for (let i = 0; i < files.length; i++) {
        const formData = new FormData();
        let file = files[i];
        formData.append('image', file);

        const progressBar = createProgressBar(file.name);
        uploadProgress.appendChild(progressBar);

        status.innerHTML = `😳 ${files[i].length+1} out of ${files.length} 😴`;

        const xhr = new XMLHttpRequest();
        xhr.open('POST', '/api/upload');

        xhr.onload = (function(file) {
            return function() {
                if (xhr.status === 200) {
                    toastr.success(`Uploaded ${file.name} successfully!`);
                } else {
                    toastr.error(`Failed to upload ${file.name}.`);
                }
                progressBar.remove();
            };
        })(file);

        xhr.onerror = (function(file) {
            return function() {
                toastr.error(`Failed to upload ${file.name}.`);
                progressBar.remove();
            };
        })(file);

        xhr.upload.addEventListener('progress', (event) => {
            if (event.lengthComputable) {
                const percentComplete = (event.loaded / event.total) * 100;
                progressBar.querySelector('.progress').style.width = percentComplete.toFixed(2) + '%';
            }
        });

        xhr.send(formData);
    }

    status.innerHTML = `Let's upload some files!`;

    fileInput.value = '';
	fileUploadLabel.textContent = 'Select files';
	uploadButton.style.display = "none";
});

function createProgressBar(fileName) {
    const progressBar = document.createElement('div');
    progressBar.classList.add('progress-bar');
    
    const progress = document.createElement('div');
    progress.classList.add('progress');
    progress.style.width = '0%';
    progress.setAttribute('aria-valuemin', '0');
    progress.setAttribute('aria-valuemax', '100');
    progressBar.appendChild(progress);
    
    const fileNameSpan = document.createElement('span');
    fileNameSpan.classList.add('file-name');
    fileNameSpan.textContent = fileName;
    progress.appendChild(fileNameSpan);
    
    return progressBar;
}


async function loadImages() {
	const response = await fetch('/api/photos');
	photos = await response.json();
	const gallery = document.getElementById('gallery');
	gallery.innerHTML = '';

	photos.forEach((photo, index) => {
		const img = document.createElement('img');
		img.src = `/api/photo/preview/${photo.filename}/2`;
		img.alt = photo.metadata;
		img.classList.add('photo');
		img.dataset.index = index;
		gallery.appendChild(img);

		img.addEventListener('click', () => openModal(index));
	});
}

function openModal(index) {
	currentPhotoIndex = index;
	const modal = document.getElementById('photo-modal');
	modal.style.display = "block";
	displayPhoto(index);

	document.onkeydown = (event) => {
		switch (event.key) {
			case "ArrowLeft":
				displayPhoto(currentPhotoIndex - 1);
				break;
			case "ArrowRight":
				displayPhoto(currentPhotoIndex + 1);
				break;
			case "Escape":
				modal.style.display = "none";
				break;
		}
	};

	const closeModal = document.getElementsByClassName('close')[0];
	closeModal.onclick = () => modal.style.display = "none";

	window.onclick = (event) => {
		if (event.target == modal) {
			modal.style.display = "none";
		}
	};
}

async function displayPhoto(index) {
    if (index < 0) index = photos.length - 1;
    if (index >= photos.length) index = 0;

    currentPhotoIndex = index;
    const media = photos[index];
    const largePhoto = document.getElementById('large-photo');
    const largeVideo = document.getElementById('large-video');
    const loadingGif = document.getElementById('loading-gif');
    const caption = document.getElementById('media-caption');
    const fileExtension = media.filename.split('.').pop().toLowerCase();

    loadingGif.style.display = 'block';
    largePhoto.style.display = 'none';
    largeVideo.style.display = 'none';

    if (fileExtension === 'jpg' || fileExtension === 'jpeg' || fileExtension === 'png' || fileExtension === 'gif') {
        largePhoto.src = `/api/photo/${media.filename}`;
        largePhoto.onload = () => {
            loadingGif.style.display = 'none';
            largePhoto.style.display = 'block';
        };
        largePhoto.alt = 'Photo';
    } else if (fileExtension === 'mp4' || fileExtension === 'webm' || fileExtension === 'mov') {
        largeVideo.src = `/api/video/${media.filename}`;
        largeVideo.onload = () => {
            loadingGif.style.display = 'none';
            largeVideo.style.display = 'block';
        };
    }

    caption.innerHTML = media.metadata;

    document.getElementById('prev-media').onclick = () => displayPhoto(currentPhotoIndex - 1);
    document.getElementById('next-media').onclick = () => displayPhoto(currentPhotoIndex + 1);
}

loadImages();