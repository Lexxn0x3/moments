let currentPhotoIndex = 0;
let photos = [];

const fileInput = document.getElementById('file-upload');
const status = document.getElementById('status');
const uploadButton = document.getElementById('upload-button');
const fileUploadLabel = document.getElementById('file-upload-label');

status.innerHTML = "Let's upload some files!";

fileInput.addEventListener('change', () => {
	if (fileInput.files.length > 0) {
		uploadButton.disabled = false;
		fileUploadLabel.textContent = `Selected ${fileInput.files.length} file(s)`;
	} else {
		uploadButton.disabled = true;
		fileUploadLabel.textContent = 'Select files';
	}
});

document.getElementById('upload-form').addEventListener('submit', async (e) => {
	e.preventDefault();
	const files = fileInput.files;
	for (let i = 0; i < files.length; i++) {
		const formData = new FormData();
		formData.append('image', files[i]);

		const response = await fetch('/api/upload', {
			method: 'POST',
			body: formData,
		});

		if (response.ok) {
			status.innerHTML += `Uploaded ${files[i].name} successfully.<br>`;
			loadImages();
		} else {
			status.innerHTML += `Failed to upload ${files[i].name}.<br>`;
		}
	}
});

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

function displayPhoto(index) {
	if (index < 0) index = photos.length - 1;
	if (index >= photos.length) index = 0;

	currentPhotoIndex = index;
	const photo = photos[index];
	const largePhoto = document.getElementById('large-photo');
	const caption = document.getElementById('photo-caption');

	largePhoto.src = `/api/photo/${photo.filename}`;
	caption.innerHTML = photo.metadata;

	document.getElementById('prev-photo').onclick = () => displayPhoto(currentPhotoIndex - 1);
	document.getElementById('next-photo').onclick = () => displayPhoto(currentPhotoIndex + 1);
}

loadImages();