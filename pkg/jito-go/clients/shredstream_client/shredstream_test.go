package shredstream_client

/*
func TestMain(m *testing.M) {
	_, filename, _, _ := runtime.Caller(0)
	godotenv.Load(filepath.Join(filepath.Dir(filename), "..", "..", "..", "jito-go", ".env"))
	os.Exit(m.Run())
}


func Test_ShredstreamClient(t *testing.T) {
	privKey, ok := os.LookupEnv("PRIVATE_KEY")
	if !assert.True(t, ok, "getting PRIVATE_KEY from .env") {
		t.FailNow()
	}

	client, err := New(jito_go.Amsterdam.ShredReceiverAddr, rpc.New(rpc.MainNetBeta_RPC), solana.MustPrivateKeyFromBase58(privKey), nil)
	if assert.NoError(t, err) {
		t.FailNow()
	}

	for {

	}
}
*/
