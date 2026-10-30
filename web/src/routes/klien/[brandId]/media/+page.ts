// Katalog media murni berbasis data klien (semua fetch di onMount) dan
// tata letak toolbarnya bergantung pada lebar layar (`isNarrow`). Server tidak
// tahu lebar layar, jadi SSR selalu merender toolbar DESKTOP dulu — di HP itu
// terlihat "toolbar desktop" muncul sekejap sebelum hidrasi menggantinya dengan
// toolbar mobile. Matikan SSR untuk rute ini supaya paint pertama langsung
// tata letak yang benar (skeleton), bukan layout desktop yang salah.
export const ssr = false;
